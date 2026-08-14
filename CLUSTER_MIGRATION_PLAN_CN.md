# sub2api 集群化改造方案

> 目标：将 sub2api 从单实例部署改造为多实例集群部署，重点解决 API 转发链路中的进程内状态、长连接亲和性、调度一致性等问题。
>
> 适用范围：`backend/` Go 服务（API 网关 + 调度 + 计费 + 后台任务）。前端为纯静态资源，天然支持多实例，不在本方案范围内。

---

## 一、当前架构现状

sub2api 已经具备相当程度的分布式基础设施，并非从零开始。下表先厘清「已就绪」与「待改造」两部分，避免重复劳动。

### 1.1 已支持分布式（无需改造）

| 模块 | 实现位置 | 分布式机制 |
|------|----------|-----------|
| 并发控制 ConcurrencyService | `repository/concurrency_cache.go` | Redis Sorted Set + Lua 脚本，使用 `Redis TIME` 避免时钟偏差 |
| RPM 限流 | `repository/rpm_cache.go` | Redis 计数器 + TxPipeline 原子递增 |
| OAuth Token 缓存 | `repository/gemini_token_cache.go`、`refresh_token_cache.go` | Redis + SETNX 分布式刷新锁 + token 版本号防竞态 |
| API Key 认证缓存 | `repository/api_key_cache.go` | Redis + pub/sub 失效广播 |
| Auth 缓存失效 | `service/auth_cache_invalidation_outbox.go` | DB outbox + Redis pub/sub |
| Sticky Session（账号粘性） | `repository/gateway_cache.go` `SetSessionAccountID` | Redis `sticky_session:{groupID}:{hash}` |
| 调度快照 SchedulerSnapshotService | `service/scheduler_snapshot_service.go` | Redis 缓存 + DB outbox 增量更新，多实例并发安全 |
| Leader Lock | `repository/leader_lock_cache.go` | Redis SETNX，已用于 token 刷新等后台任务 |
| Session Binding（IP/UA 指纹） | `service/session_binding.go` | 无服务端存储，指纹在 JWT 中携带 |

**结论：API 转发的主干链路（鉴权 → 限流 → 并发槽位 → sticky 选号 → 转发 → 计费）已基本分布式化。** 集群化的剩余障碍集中在「调度决策的进程内辅助状态」「WebSocket 长连接亲和性」「若干进程内缓存」三类。

### 1.2 待改造的进程内状态

下面按对 API 转发的影响程度分级。

---

## 二、集群化障碍清单（按优先级）

### P0 — 必须改造，否则转发行为异常

#### 2.1 DigestSessionStore（摘要会话存储）
- **位置**：`service/digest_session_store.go`
- **现状**：使用进程内 `gocache.Cache`，key 为 `{groupID}:{prefixHash}|{digestChain}` → `{uuid, accountID}`，TTL 5 分钟。
- **用途**：Anthropic/OpenAI 兼容层在请求携带 `digest_chain` 时，用于把同一对话链路绑定到同一上游账号（digest-based sticky）。
- **集群问题**：客户端的后续请求被负载均衡打到另一实例时，digest 会话丢失 → 重新选号 → 上游账号切换 → 可能触发上游会话断裂或重复计费。
- **改造方案**：见 §3.1。

#### 2.2 OpenAI/Grok 网关运行时状态（sync.Map 集合）
- **位置**：`service/openai_gateway_service.go` 行 451-465
- **现状**：
  ```go
  openaiWSFallbackUntil               sync.Map // 账号 WS 回退截止时间
  openaiAccountRuntimeBlockUntil      sync.Map // 账号运行时阻塞截止
  openaiAccountRuntimeBlockLocks      sync.Map // 阻塞锁（进程内）
  openaiAccountRuntimeBlockGeneration sync.Map // 阻塞代数
  grokCredentialMutationLocks         sync.Map // Grok 凭证变更锁
  openaiCompatSessionResponses        sync.Map // OpenAI 兼容会话响应缓存
  openaiCompatAnthropicDigestSessions sync.Map // Anthropic 摘要会话（另一份）
  ```
- **集群问题**：
  - `openaiWSFallbackUntil` / `openaiAccountRuntimeBlockUntil`：A 实例发现账号异常并设置回退，B 实例不知道，仍会把请求调度到该账号 → 上游错误率上升。
  - `grokCredentialMutationLocks`：进程内互斥锁，多实例同时变更 Grok 凭证会导致写冲突/重复刷新。
  - `openaiCompatSessionResponses` / `openaiCompatAnthropicDigestSessions`：会话级响应缓存，跨实例丢失导致兼容层行为不一致。
- **改造方案**：见 §3.2。

#### 2.3 OpenAI 账号运行时统计
- **位置**：`service/openai_account_scheduler.go` 行 180-290，`openAIAccountRuntimeStats.accounts sync.Map`
- **现状**：进程内维护每账号的错误率、TTFT、连续失败数等运行时指标，用于调度打分。
- **集群问题**：每个实例只看到「打到自己身上的请求」的统计，调度打分基于局部数据 → 不同实例对同一账号的健康度判断不一致 → 负载不均、故障账号未被全局熔断。
- **改造方案**：见 §3.3。

#### 2.4 WebSocket 长连接池（最大障碍）
- **位置**：`service/openai_ws_pool.go`（`newOpenAIWSConnPool`），以及 `openai_ws_forwarder_*.go` 系列
- **现状**：OpenAI Realtime / Responses WebSocket 走进程内连接池，每个账号维护若干到上游的长连接，请求通过 `openAIWSConnLease` 租用连接。连接与实例强绑定。
- **集群问题**：
  1. **连接不可迁移**：WS 长连接是 TCP 级有状态资源，无法在实例间转移。
  2. **会话亲和性**：同一 Realtime 会话的多个帧必须落到同一实例（同一上游连接），否则会话断裂。
  3. **预热连接浪费**：每实例独立预热，N 实例 = N 倍上游连接数，可能撞上游连接上限。
- **改造方案**：见 §3.4。

### P1 — 建议改造，影响调度精度与配额准确性

#### 2.5 Grok 免费配额门禁
- **位置**：`service/grok_free_quota_gate.go` 行 127-131
- **现状**：`gatewayGrokFreeQuotaGateCache`、`openaiGrokFreeQuotaGateCache`、`freeQuotaRefreshInFlight` 三个全局 `sync.Map`。
- **集群问题**：免费配额门禁状态不共享，多实例可能同时放行超额请求，触发上游硬性拒绝。
- **改造方案**：见 §3.5。

#### 2.6 accountWriteThrottle（Codex 快照持久化节流）
- **位置**：`service/openai_gateway_service.go` 行 361-399，`defaultOpenAICodexSnapshotPersistThrottle`
- **现状**：进程内 `map[int64]time.Time` 节流，限制每账号快照写入频率。
- **集群问题**：N 实例 = N 倍实际写入频率，可能对 DB 造成写压力或触发上游限速。
- **改造方案**：见 §3.6。

#### 2.7 Gemini 使用量预检查缓存
- **位置**：`service/ratelimit_service.go` 行 21-35，`usageCache map[int64]*geminiUsageCacheEntry`
- **现状**：进程内 map，TTL 1 分钟，缓存 Gemini 配额预检查结果。
- **集群问题**：预检查结果不共享，多实例重复查询上游配额 → 上游 QPS 放大。
- **改造方案**：见 §3.7。

#### 2.8 账号使用统计缓存
- **位置**：`service/account_usage_service.go` 行 121-127
- **现状**：`apiCache`、`windowStatsCache`、`antigravityCache`、`openAIProbeCache`、`grokProbeCache` 五个 `sync.Map`。
- **集群问题**：窗口费用/配额判断基于局部缓存，可能误判账号可调度性。
- **改造方案**：见 §3.8。

### P2 — 可选改造，影响有限

| 模块 | 位置 | 说明 |
|------|------|------|
| User L1 活跃时间缓存 | `service/user_service.go:296` `lastActiveTouchL1` | 写缓冲性质，丢失影响小，多实例各自批量写即可 |
| APIKey L1 使用时间缓存 | `service/api_key_service.go:310` `lastUsedTouchL1` | 同上 |
| Billing fallback 警告标记 | `service/billing_service.go:181` `fallbackWarnSeen` | 仅日志去重，多实例重复告警可接受 |
| Agent Identity 任务锁 | `service/openai_agent_identity.go:32` `agentIdentityTaskLocks` | 短任务去重，多实例偶发重复可接受 |
| Grok 观察模型飞行锁 | `service/grok_observed_models.go:29` | 同上 |
| Antigravity 回填冷却 | `service/antigravity_token_provider.go:31` `backfillCooldown` | 冷却窗口略缩短即可 |

---

## 三、改造方案（分模块）

### 3.1 DigestSessionStore → Redis 实现

**目标**：digest 会话跨实例共享。

**步骤**：
1. 新增 `repository/digest_session_cache.go`，提供与 `DigestSessionStore` 相同的 `Save`/`Find` 接口，底层用 Redis。
   - key 格式：`digest_session:{groupID}:{prefixHash}|{digestChain}`
   - value：JSON `{uuid, accountID}`
   - TTL：5 分钟（与现有一致）
2. `Find` 的「逐段截断最长匹配」逻辑：用 Redis `SCAN` 或在 value 中额外存储 chain 列表。推荐方案：写入时同时写一份 `digest_session_index:{groupID}:{prefixHash}` (SET) 记录该命名空间下所有 chain，`Find` 时先 `SMEMBERS` 再按长度排序匹配。若担心 `SCAN` 性能，可改为只存最长 chain（牺牲精度换简单）。
3. `wire.go` 中把 `NewDigestSessionStore()` 替换为新的 Redis 实现。
4. 保留进程内 L1 缓存作为热点加速（可选）：`DigestSessionStore` 内部加一层 `gocache`，miss 时回 Redis；`Save` 时双写。L1 TTL 设为 10-15 秒，避免长期不一致。

**风险**：Redis 网络延迟比进程内内存高 ~1ms，对热路径有轻微影响。可通过 L1 缓解。

### 3.2 OpenAI/Grok 运行时状态 → Redis

**目标**：账号级回退/阻塞状态全局可见。

**按字段拆分**：

| 字段 | Redis 方案 |
|------|-----------|
| `openaiWSFallbackUntil` | `openai:ws_fallback:{accountID}` = 截止时间戳，TTL = 回退时长 |
| `openaiAccountRuntimeBlockUntil` | `openai:runtime_block:{accountID}` = 截止时间戳，TTL = 阻塞时长 |
| `openaiAccountRuntimeBlockGeneration` | 与 block key 合并，value 存代数；或单独 key `openai:runtime_block_gen:{accountID}` |
| `openaiAccountRuntimeBlockLocks` | 改用 Redis 分布式锁 `openai:runtime_block_lock:{accountID}` SETNX |
| `grokCredentialMutationLocks` | 改用 Redis 分布式锁 `grok:cred_mutation_lock:{accountID}` |
| `openaiCompatSessionResponses` | `openai:compat_session_resp:{sessionKey}` JSON，TTL 与现有一致 |
| `openaiCompatAnthropicDigestSessions` | 与 §3.1 合并，统一走 Redis digest session |

**步骤**：
1. 在 `repository/` 新增 `openai_runtime_cache.go`，封装上述 key 的 Get/Set/Lock 操作。
2. `openai_gateway_service.go` 中的 sync.Map 访问点替换为 Redis 调用。
3. 对高频读（如 `openaiWSFallbackUntil` 在每次选号时检查），加进程内 L1 短 TTL 缓存（2-3 秒），降低 Redis QPS。

**风险**：阻塞锁从进程内 mutex 改为 Redis 锁，临界区性能略降。但阻塞操作本身不频繁，可接受。

### 3.3 账号运行时统计 → 聚合方案

**目标**：多实例共享账号健康度指标，调度打分一致。

**两种方案**：

**方案 A（推荐）：Redis 共享计数器 + 进程内滑动窗口**
- 错误计数、请求数等用 Redis `INCR` + TTL 窗口（参考 `rpm_cache.go` 实现）。
- TTFT 等时延指标用 Redis `HISTOGRAM`（或简化为 P50/P95 桶）。
- 每实例仍维护本地近期原始数据用于快速打分，Redis 数据作为全局兜底（周期性同步）。

**方案 B：Leader 聚合**
- 用现有 `LeaderLockCache` 选出 leader，leader 周期性从所有实例收集指标（通过 Redis pub/sub 或 DB 汇总表）。
- 仅 leader 的调度打分生效，follower 走 leader 下发的「账号健康度快照」。
- 缺点：leader 单点，故障切换期间调度精度下降。

**推荐方案 A**，因为 sub2api 已有成熟的 Redis 计数器模式（RPM），复用成本低。

### 3.4 WebSocket 长连接池 — 集群化最难点

**核心矛盾**：WS 长连接是有状态资源，无法跨实例迁移；但 Realtime 会话需要连接亲和性。

**推荐方案：会话亲和路由 + 连接池分片**

1. **入口层会话亲和**（必须）：
   - 在负载均衡器（Nginx/ALB）层，对 `/v1/realtime` 等 WS 升级请求按 `session_id`（或 `previous_response_id`）做一致性哈希，把同一会话路由到同一实例。
   - 对首次请求（无 session_id）走普通负载均衡。
   - 这样 WS 连接天然粘在单实例，无需跨实例共享连接池。

2. **连接池分片**：
   - 每实例只为自己承接的会话维护连接池，不再尝试全局预热。
   - 配置项 `gateway.openai_ws.max_conns_per_account` 改为「每实例上限」，部署时按 `实例数 × 上限 ≤ 上游允许总连接数` 规划。
   - 预热逻辑（`EnsureTargetIdleAsync`）保留，但目标 idle 数按实例数缩放。

3. **故障转移**：
   - 实例宕机时，其 WS 连接全部断开，客户端会重连 → LB 重新哈希到存活实例 → 新建上游连接。
   - 需确保客户端有重连逻辑（Realtime SDK 通常自带）。
   - 服务端侧：`openaiCompatSessionResponses` 若已迁移到 Redis（§3.2），新实例可恢复会话上下文。

4. **非 WS 转发不受影响**：普通 HTTP 转发（SSE 流式）无连接亲和性需求，多实例随机调度即可。

**替代方案（不推荐）**：把 WS 转发拆成独立的有状态服务集群，HTTP 网关集群通过内部 RPC 调用 WS 服务。改动大、引入额外网络跳数，仅在 WS 流量占比极高时考虑。

### 3.5 Grok 免费配额门禁 → Redis

**步骤**：
1. 新增 `repository/grok_free_quota_cache.go`：
   - `grok:free_quota_gate:{platform}:{accountID}` = JSON 状态，TTL 与现有一致。
   - `grok:free_quota_refresh_lock:{accountID}` SETNX 防并发刷新。
2. `grok_free_quota_gate.go` 中的三个 `sync.Map` 替换为 Redis 调用。
3. `freeQuotaRefreshInFlight` 改为 Redis 分布式锁。

### 3.6 accountWriteThrottle → Redis 滑动窗口

**步骤**：
- 用 Redis `SETNX` + TTL 实现分布式节流：`openai:codex_snapshot_throttle:{accountID}` = 1，TTL = `minInterval`。
- 写入前 `SETNX`，返回 0 则跳过本次持久化。
- 注意：Redis SETNX 的 TTL 精度为毫秒级，比进程内 map 略粗，对节流场景足够。

### 3.7 Gemini 预检查缓存 → Redis

**步骤**：
- 新增 `repository/gemini_usage_precheck_cache.go`：
  - `gemini:usage_precheck:{accountID}` = JSON `{quota, checked_at}`，TTL 1 分钟。
- `ratelimit_service.go` 的 `usageCache` 替换为 Redis 调用。
- 可保留进程内 L1（10 秒）降低 Redis QPS。

### 3.8 账号使用统计缓存 → Redis

**步骤**：
- `apiCache`、`windowStatsCache` 等 5 个 sync.Map 迁移到 Redis，key 格式 `account_usage:{type}:{accountID}`，TTL 与现有一致。
- `openAIProbeCache`、`grokProbeCache`（探测冷却）用 Redis SETNX + TTL。
- 数据量较大时考虑用 Redis Hash 结构批量读取。

---

## 四、分阶段实施路线

### 阶段 1：低风险基础设施（1-2 周）
- §3.1 DigestSessionStore → Redis
- §3.5 Grok 免费配额门禁 → Redis
- §3.6 accountWriteThrottle → Redis
- §3.7 Gemini 预检查缓存 → Redis
- §3.8 账号使用统计缓存 → Redis

**验证**：单实例功能回归测试 + 双实例灰度，观察缓存命中率与一致性。

### 阶段 2：调度状态共享（1-2 周）
- §3.2 OpenAI/Grok 运行时状态 → Redis
- §3.3 账号运行时统计 → Redis 聚合

**验证**：双实例下手动触发账号故障，观察另一实例是否同步熔断。

### 阶段 3：WebSocket 集群化（2-3 周）
- §3.4 会话亲和路由 + 连接池分片
- LB 配置一致性哈希（按 session_id）
- 连接池上限按实例数重新规划

**验证**：多实例下 WS 会话连续性测试、实例宕机切换测试。

### 阶段 4：后台任务 Leader 化（1 周）
- 审计所有后台周期任务（token 刷新、配额探测、清理任务等），确认是否已用 `LeaderLockCache`。
- 未走 leader 的任务改为 leader-only，避免多实例重复执行。
- 检查 `cmd/server/main.go` 启动的 cron job 列表。

### 阶段 5：部署与验证（1 周）
- 见 §五、§六。

---

## 五、部署架构建议

```
                    ┌─────────────────────────┐
                    │   负载均衡器 (Nginx/ALB)  │
                    │  - HTTP: 轮询/最少连接    │
                    │  - WS: session_id 一致性哈希│
                    └───────────┬─────────────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          │                     │                     │
   ┌──────▼──────┐       ┌──────▼──────┐       ┌──────▼──────┐
   │  实例 1     │       │  实例 2     │       │  实例 N     │
   │  sub2api    │       │  sub2api    │       │  sub2api    │
   │  (无状态+   │       │  (无状态+   │       │  (无状态+   │
   │   WS亲和)   │       │   WS亲和)   │       │   WS亲和)   │
   └──────┬──────┘       └──────┬──────┘       └──────┬──────┘
          │                     │                     │
          └──────────┬──────────┴──────────┬──────────┘
                     │                     │
              ┌──────▼──────┐       ┌──────▼──────┐
              │  PostgreSQL │       │    Redis    │
              │  (共享DB)   │       │  (共享缓存/ │
              │             │       │   锁/限流)  │
              └─────────────┘       └─────────────┘
```

**关键配置**：
- **LB 健康检查**：`/health` 端点，检查 DB + Redis 连通性。
- **会话亲和**：仅对 WS 升级请求（`Upgrade: websocket`）启用，HTTP 请求保持普通负载均衡。
- **实例数规划**：WS 连接数上限 = `实例数 × gateway.openai_ws.max_conns_per_account` ≤ 上游允许总连接数。
- **滚动更新**：WS 实例下线前需等待连接自然结束或主动关闭，建议设置 `max_fails` + `fail_timeout` 让 LB 逐步摘除。
- **Redis 容量**：迁移后 Redis 内存占用上升，需评估 key 数量并扩容。digest session、运行时统计等 key 数量 ≈ `账号数 × 实例数` 量级。

---

## 六、验证方案

### 6.1 单实例回归
每个阶段完成后，先在单实例下跑完整测试套件，确保功能无回归：
```bash
cd backend && go test ./internal/service/... ./internal/repository/...
```

### 6.2 双实例一致性测试
1. 启动两个实例，共享同一 PG + Redis。
2. 用同一 API Key 发起连续请求（带 session_id），观察：
   - sticky session 是否在两实例间正确命中（Redis 共享）。
   - digest session 是否跨实例一致。
   - 账号故障熔断是否在另一实例同步生效。
3. 用 `hey`/`wrk` 压测，观察并发计数是否准确（不超过账号 concurrency 上限）。

### 6.3 WS 会话连续性测试
1. 客户端建立 Realtime WS 连接，连续发送多帧。
2. 期间手动重启非当前实例，观察会话是否中断（应不中断）。
3. 重启当前实例，观察客户端重连后是否落到新实例并恢复上下文。

### 6.4 故障注入
- 杀掉一个实例的 Redis 连接，观察是否优雅降级（L1 缓存兜底 + 报警）。
- 杀掉 leader 实例，观察后台任务是否被新 leader 接管。

---

## 七、风险与回滚

| 风险 | 影响 | 缓解 |
|------|------|------|
| Redis 延迟引入热路径开销 | 转发 P99 延迟上升 1-3ms | 关键路径加进程内 L1 短 TTL 缓存 |
| Redis 故障导致全局不可用 | 全集群停摆 | Redis Sentinel/Cluster 高可用 + L1 兜底 |
| WS 会话亲和配置错误 | 会话频繁断裂 | 灰度上线，先在测试环境验证 LB 哈希策略 |
| 迁移期间状态不一致 | 部分实例旧逻辑、部分新逻辑 | 分阶段灰度，每阶段全量切换后再进入下一阶段 |
| Redis 内存增长 | OOM | 监控 key 数量，合理设 TTL，必要时扩容 |

**回滚策略**：每个改造点通过 feature flag（`config.Gateway.ClusterMode`）控制，出问题可单点回退到进程内实现。

---

## 八、改造点速查表

| 优先级 | 模块 | 文件 | 改造方向 |
|--------|------|------|---------|
| P0 | DigestSessionStore | `service/digest_session_store.go` | → Redis + L1 |
| P0 | OpenAI/Grok 运行时状态 | `service/openai_gateway_service.go:451-465` | → Redis + 分布式锁 |
| P0 | 账号运行时统计 | `service/openai_account_scheduler.go:180-290` | → Redis 计数器 |
| P0 | WebSocket 连接池 | `service/openai_ws_pool.go` | LB 会话亲和 + 分片 |
| P1 | Grok 免费配额门禁 | `service/grok_free_quota_gate.go:127-131` | → Redis |
| P1 | accountWriteThrottle | `service/openai_gateway_service.go:361-399` | → Redis SETNX |
| P1 | Gemini 预检查缓存 | `service/ratelimit_service.go:21-35` | → Redis + L1 |
| P1 | 账号使用统计缓存 | `service/account_usage_service.go:121-127` | → Redis |
| P2 | User/APIKey L1 | `service/user_service.go:296` 等 | 可不改造 |
| P2 | Billing/Agent 杂项锁 | 多处 sync.Map | 可不改造 |

---

## 九、结论

sub2api 的核心转发链路（鉴权、限流、并发、sticky 选号、调度快照）已具备分布式能力，集群化改造的工作量集中在**辅助状态的 Redis 化**和 **WebSocket 长连接的会话亲和路由**两块。

- **HTTP 转发**：改造量中等，P0+P1 共 8 个模块，预计 4-6 周。
- **WebSocket 转发**：无需改代码，主要是 LB 配置 + 连接池容量规划，但验证成本高。
- **最大收益**：水平扩展能力 + 单实例故障不影响全局。
- **最大风险**：Redis 成为强依赖，需保证 Redis 高可用。
