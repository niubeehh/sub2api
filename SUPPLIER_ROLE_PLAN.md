# 供应商角色（Supplier Role）实施方案

## 一、背景与目标

当前系统用户只有两种角色：普通用户（`user`）和管理员（`admin`）。需要新增**供应商角色（`supplier`）**，用于：

- 提供上游账号（Account），账号归属到具体供应商
- 供应商可查看自己名下每个账号的消耗、整体使用等数据
- 供应商可自建/导入上游账号（owner_id 自动 = 自己）
- 供应商为纯观测角色，不参与计费/扣费链路

## 二、决策点（已确认）

| 决策 | 选择 |
|---|---|
| 账号归属来源 | 供应商可自建（owner_id 自动 = 自己），管理员可全局管理 |
| 数据可见范围 | 账号 + 下游明细（含 user_id / api_key_id） |
| 权限模型 | 独立路由树 `/api/v1/supplier/*` + SupplierAuth 中间件 |
| 供应商定位 | 纯观测，不涉及余额/计费 |

## 三、核心设计

### 3.1 数据模型

- `accounts` 表新增 `owner_id BIGINT NULL`（NULL = 平台托管，兼容存量）
- 新增 `idx_accounts_owner_id` 部分索引
- User.role 新增 `supplier` 取值

### 3.2 鉴权

- 新建 `SupplierAuth` 中间件：允许 `admin` / `supplier` 通过
- supplier 通过时，把 `owner_id = user.ID` 注入 context 作为强制过滤约束
- admin 通过时不注入（看全部）
- 现有 `adminAuth` 不动（admin 路由仍只允许 admin）

### 3.3 路由

- 新建 `/api/v1/supplier/*` 路由树
- 复用底层 service（AdminService、AccountUsageService、UsageService、各 OAuth service）
- handler 内强制按 context owner_id 过滤

### 3.4 OAuth 流程

OAuth 流程是两步分离的（generate-auth-url → exchange-code 只换 token 不建账号，账号创建在 `AccountHandler.Create`），因此 **owner_id 绑定无风险**：供应商版 Create handler 直接从 context 取 user.ID 写入即可。

## 四、分阶段实施计划

### 阶段一：角色 + owner_id 字段 + 管理员分配 + supplier 只读观测

**目标**：供应商能登录、能看到自己名下账号的消耗数据；管理员能给账号分配归属。

**状态：已完成 ✅**

- [x] 1.1 角色常量：`domain/constants.go` + `service/domain_constants.go` 新增 `RoleSupplier`
- [x] 1.2 ent schema：`account.go` 新增 `owner_id` 字段 + 索引
- [x] 1.3 运行 `go generate ./ent` 重新生成 ent 代码
- [x] 1.4 新增 SQL 迁移 `221_add_account_owner_id.sql`
- [x] 1.5 `normalizeUserRole` 接受 `supplier`；新增 `User.IsSupplier()`
- [x] 1.6 `service.Account` 结构体 + `dto.Account` 增加 OwnerID 字段；repo 转换补齐
- [x] 1.7 `AccountRepository` 新增 `ListByOwnerWithFilters` + `ListAllByOwnerWithFilters`（不改原签名，避免破坏现有调用方）
- [x] 1.8 管理员侧 `CreateAccountRequest`/`UpdateAccountRequest` 增加 `OwnerID`；`CreateAccountInput`/`UpdateAccountInput` 增加 `OwnerID`；handler 透传
- [x] 1.9 `SupplierOnly` 中间件 + `ContextKeyOwnerFilter`（admin 放行不注入过滤，supplier 注入 owner_id）
- [x] 1.10 Supplier 路由树 `/api/v1/supplier/*` + 只读 handler（accounts 列表/详情、stats、usage、today-stats、dashboard、usage 列表）+ wire 注入
- [x] 1.11 `UsageLogFilters` 新增 `AccountIDs` 字段；repo `ListWithFilters`/`GetStatsWithFilters` 支持 `account_id = ANY($n)` 数组过滤
- [x] 1.12 前端：auth store 新增 `isSupplier`；router meta 新增 `requiresSupplier` + 守卫；侧边栏 supplier 菜单
- [x] 1.13 前端：supplier 视图（DashboardView、AccountsView、AccountDetailView、UsageView）+ supplier API 模块
- [x] 1.14 前端：admin AccountsView 增加归属列；UsersView 角色过滤下拉加 supplier；UserEditModal 角色选择加 supplier；Account/CreateAccountRequest/UpdateAccountRequest 类型加 owner_id

### 阶段二：供应商自建账号 + OAuth 导入

**目标**：供应商可在自己名下创建/导入上游账号。

**状态：已完成 ✅**

- [x] 2.1 Supplier 账号 CRUD handler（Create 强制 owner_id = 自己，PUT/DELETE 校验归属）
- [x] 2.2 Supplier OAuth 流程端点（generate-auth-url、exchange-code、cookie-auth 等转发复用）
- [x] 2.3 Supplier 账号导入端点（codex-session 等 + owner 注入）——委托 admin AccountHandler.ImportCodexSessionForOwner
- [x] 2.4 Supplier 账号操作端点（test、refresh、clear-error、clear-rate-limit 等 + owner 校验）
- [x] 2.5 前端：supplier AccountsView 增加创建/导入入口；AccountDetailView 增加操作按钮

## 五、改动规模评估

| 模块 | 文件数 | 改动量 | 风险 |
|---|---|---|---|
| 角色常量 + ent schema + 迁移 | ~4 | 小 | 低 |
| 管理员侧账号归属字段 | ~10 | 中 | 中（repo 签名） |
| Supplier 鉴权中间件 | 1 新增 | 小 | 低 |
| Supplier 路由 + handler（阶段一） | ~6 新增 | 中 | 低 |
| Supplier 路由 + handler（阶段二） | ~6 新增 | 中 | 低 |
| UsageService owner 聚合方法 | ~3 | 小-中 | 低 |
| 前端 supplier 视图 + 路由 + store | ~10 新增 | 中 | 低 |
| 前端 admin 视图角色扩展 | ~3 | 小 | 低 |

## 六、进度记录

### 2026-08-14
- 创建实施方案文档
- 切换到新分支 `feat/supplier-role`
- 开始阶段一实施

### 阶段一完成
- 后端：角色常量、ent schema、迁移、service/repo/handler owner 字段、SupplierOnly 中间件、supplier 路由树 + 只读 handler、UsageLogFilters.AccountIDs 支持
- 前端：auth store isSupplier、router 守卫、侧边栏 supplier 菜单、supplier 视图 4 个页面 + API 模块、admin 视图角色扩展
- 验证：
  - `go build ./...` ✅
  - `go test ./...` ✅（全部通过）
  - `npm run typecheck` ✅（前端类型检查通过）
- 修改文件清单：
  - 后端：`domain/constants.go`、`service/domain_constants.go`、`service/user.go`、`service/admin_user.go`、`service/account.go`、`service/account_service.go`、`service/admin_service.go`、`service/admin_account.go`、`repository/account_repo.go`、`repository/usage_log_repo_query.go`、`repository/usage_log_repo_stats.go`、`handler/dto/types.go`、`handler/dto/mappers.go`、`handler/admin/account_handler.go`、`handler/handler.go`、`handler/wire.go`、`server/middleware/middleware.go`、`server/middleware/supplier_only.go`、`server/routes/supplier.go`、`server/router.go`、`ent/schema/account.go` + ent 生成文件、`migrations/221_add_account_owner_id.sql`、`cmd/server/wire_gen.go`
  - 后端测试 mock 补齐：`api_contract_test.go`、`ratelimit_session_window_test.go`、`gemini_multiplatform_test.go`、`gateway_multiplatform_test.go`、`account_service_delete_test.go`、`admin_service_stub_test.go`、`account_repo_upstream_billing_probe_update_test.go`
  - 前端：`stores/auth.ts`、`router/meta.d.ts`、`router/index.ts`、`api/index.ts`、`api/supplier.ts`、`types/index.ts`、`components/layout/AppSidebar.vue`、`components/admin/user/UserEditModal.vue`、`views/admin/UsersView.vue`、`views/admin/AccountsView.vue`、`views/supplier/DashboardView.vue`、`views/supplier/AccountsView.vue`、`views/supplier/AccountDetailView.vue`、`views/supplier/UsageView.vue`

### 阶段二完成
- 后端：
  - supplier AccountHandler 扩展写操作：Create（强制 owner_id）、Update/Delete（校验归属）、Test、Refresh、ClearError、ClearRateLimit
  - supplier OAuthHandler：GenerateAuthURL、ExchangeCode、CookieAuth 等 6 个端点，复用 OAuthService
  - Codex session 导入：admin AccountHandler 新增 `ImportCodexSessionForOwner` 方法，supplier 委托调用，强制 owner_id 注入 + owner 范围去重
  - supplier 路由树新增写操作 + OAuth + 导入端点
  - wire 注入更新（supplier.NewOAuthHandler、adminAccountHandler 依赖）
- 前端：
  - supplier API 扩展：createAccount、updateAccount、deleteAccount、testAccount、refreshAccount、clearAccountError、clearAccountRateLimit、OAuth 6 个方法、importCodexSession
  - supplier AccountsView 增加创建账号弹窗 + 导入 Codex 弹窗 + 行内操作按钮（清除错误、刷新、删除）
  - supplier AccountDetailView 增加操作按钮（刷新凭据、清除错误、清除限流、删除账号）
- 验证：
  - `go build ./...` ✅
  - `go test ./...` ✅（全部通过）
  - `npm run typecheck` ✅
- 新增/修改文件：
  - 后端：`handler/supplier/account_handler.go`（扩展依赖）、`handler/supplier/account_write_handler.go`（新增写操作 + OAuth handler）、`handler/admin/account_codex_import.go`（ImportCodexSessionForOwner + ownerID 参数）、`handler/handler.go`（SupplierHandlers.OAuth）、`handler/wire.go`（supplier.NewOAuthHandler）、`server/routes/supplier.go`（写操作路由）、`cmd/server/wire_gen.go`
  - 后端测试修复：`account_codex_agent_identity_import_test.go`、`account_codex_import_test.go`（importCodexSessions 加 nil 参数）
  - 前端：`api/supplier.ts`（写操作 + OAuth + 导入 API）、`views/supplier/AccountsView.vue`（创建/导入弹窗 + 操作按钮）、`views/supplier/AccountDetailView.vue`（操作按钮）

### 阶段三：使用记录 + 代理管理 + Dashboard 增强 ✅ 已完成

**目标**：供应商能管理自己的代理、查看完整使用记录、Dashboard 展示归属账号消费明细。

**背景**：万级账号场景下，按 account_ids 数组过滤 usage_log 性能不可接受，需要冗余 `account_owner_id` 字段。

**状态**：全部完成，编译通过，测试通过。

#### 3.A usage_log 加 account_owner_id 冗余字段 ✅

- [x] 3.A.1 ent schema：`usage_log.go` 新增 `account_owner_id` 字段 + 部分索引
- [x] 3.A.2 运行 `go generate ./ent`
- [x] 3.A.3 新增 SQL 迁移 `222_add_usage_log_account_owner_id.sql`（含历史数据回填）
- [x] 3.A.4 service.UsageLog 结构体新增 AccountOwnerID 字段
- [x] 3.A.5 UsageLogFilters 新增 AccountOwnerID 字段（与 AccountIDs 互斥，优先使用）
- [x] 3.A.6 写入路径注入：`gateway_usage_billing.go` buildRecordUsageLog 从 account.OwnerID 取值
- [x] 3.A.7 repository insert 写入 account_owner_id
- [x] 3.A.8 repository query/stats 支持 AccountOwnerID 过滤
- [x] 3.A.9 账号归属变更时同步更新 usage_logs.account_owner_id（admin UpdateAccount）
- [x] 3.A.10 supplier 查询改用 AccountOwnerID（不再先查 account_ids 再 ANY 数组）

#### 3.B 供应商代理管理 ✅

- [x] 3.B.1 ent schema：`proxy.go` 新增 `owner_id` 字段 + 索引
- [x] 3.B.2 运行 `go generate ./ent`
- [x] 3.B.3 新增 SQL 迁移 `223_add_proxy_owner_id.sql`
- [x] 3.B.4 service.Proxy 结构体 + dto 新增 OwnerID
- [x] 3.B.5 ProxyRepository 新增 ListByOwner / GetByIDAndOwner / ListActiveByOwner / ListByOwnerWithAccountCount
- [x] 3.B.6 supplier proxy handler 直接使用 ProxyRepository owner 过滤方法（不走 AdminService）
- [x] 3.B.7 supplier proxy handler：CRUD + 测试 + 质量检查，强制 owner 注入和归属校验
- [x] 3.B.8 supplier 路由注册 `/supplier/proxies/*`
- [x] 3.B.9 账号创建时校验 proxy_id 归属（供应商不能绑定别人的代理）
- [x] 3.B.10 wire 注入更新（NewProxyHandler + NewAccountHandler 加 proxyRepo）
- [x] 3.B.11 前端：supplier API 新增代理方法
- [x] 3.B.12 前端：supplier ProxiesView 新增（精简版，含创建/编辑/删除/测试）
- [x] 3.B.13 前端：router + 侧边栏新增代理菜单

#### 3.C 使用记录增强 ✅

- [x] 3.C.1 supplier ListUsage 补充 date_range / billing_mode / request_type 等过滤条件
- [x] 3.C.2 新增 `GET /supplier/usage/stats` 端点（使用记录统计聚合）
- [x] 3.C.3 supplier 查询改用 AccountOwnerID（依赖 3.A 完成）
- [x] 3.C.4 前端 UsageView 增加日期范围选择 + 统计卡片
- [x] 3.C.5 前端 supplier API 新增 usage stats 方法

#### 3.D Dashboard 增强 ✅

- [x] 3.D.1 ~~supplier AccountHandler 注入 concurrencyService / sessionLimitCache / rpmCache~~（P1，暂不做）
- [x] 3.D.2 ~~GetByID 返回运行时计费信息~~（P1，暂不做）
- [x] 3.D.3 新增 `POST /supplier/accounts/usage/batch` 端点（批量查账号用量）
- [x] 3.D.3b 新增 `POST /supplier/accounts/today-stats/batch` 端点（批量查今日统计）
- [x] 3.D.4 Dashboard 改用 AccountOwnerID 聚合（依赖 3.A 完成）
- [x] 3.D.5 前端 DashboardView 增加账号消耗明细列表
- [x] 3.D.6 ~~前端 AccountDetailView 显示运行时计费信息~~（P1，暂不做）
- [x] 3.D.7 前端 supplier API 新增 batch usage 方法

### 阶段三验证结果

```bash
# 后端编译
cd backend && go build ./...    # ✅ 通过

# 后端测试
cd backend && go test ./...     # ✅ 全部通过

# 前端类型检查
cd frontend && npm run typecheck # ✅ 通过
```

### 阶段三变更文件清单

**后端**：
- `backend/ent/schema/usage_log.go` — 新增 account_owner_id 字段
- `backend/ent/schema/proxy.go` — 新增 owner_id 字段
- `backend/migrations/222_add_usage_log_account_owner_id.sql` — usage_log 迁移
- `backend/migrations/223_add_proxy_owner_id.sql` — proxy 迁移
- `backend/internal/service/proxy.go` — Proxy.OwnerID + ProxyAccountSummary.OwnerID
- `backend/internal/service/proxy_service.go` — ProxyRepository 接口新增 owner 方法
- `backend/internal/repository/proxy_repo.go` — owner 过滤方法 + Create/EntityToService 映射
- `backend/internal/repository/usage_log_repo_insert.go` — 写入 account_owner_id
- `backend/internal/repository/usage_log_repo_query.go` — scanUsageLog 支持 account_owner_id
- `backend/internal/repository/usage_log_repo_stats.go` — owner 过滤
- `backend/internal/handler/supplier/proxy_handler.go` — 新增供应商代理 handler
- `backend/internal/handler/supplier/account_handler.go` — ListUsage 增强 + GetUsageStats + GetBatchUsage + GetBatchTodayStats
- `backend/internal/handler/supplier/account_write_handler.go` — 账号创建校验 proxy_id 归属
- `backend/internal/handler/handler.go` — SupplierHandlers 加 Proxy
- `backend/internal/handler/wire.go` — wire 注入
- `backend/internal/server/routes/supplier.go` — supplier 代理路由 + usage/stats + batch 端点
- `backend/cmd/server/wire_gen.go` — wire 生成代码更新
- `backend/internal/service/account_usage_service_batch_test.go` — 测试 stub 补全
- `backend/internal/service/content_moderation_proxy_test.go` — 测试 stub 补全
- `backend/internal/repository/usage_log_repo_request_type_test.go` — 测试参数对齐

**前端**：
- `frontend/src/api/supplier.ts` — 新增代理管理 + batch usage + usage stats API
- `frontend/src/views/supplier/ProxiesView.vue` — 新增供应商代理管理页
- `frontend/src/components/supplier/ProxyFormModal.vue` — 新增代理表单弹窗
- `frontend/src/views/supplier/UsageView.vue` — 增加日期范围 + 统计卡片
- `frontend/src/views/supplier/DashboardView.vue` — 增加账号消耗明细列表
- `frontend/src/router/index.ts` — 新增 /supplier/proxies 路由
- `frontend/src/components/layout/AppSidebar.vue` — 侧边栏新增代理菜单

### 阶段三范围排除（P1，后续迭代）

- supplier AccountDetail 运行时计费信息（窗口消耗 / 活跃会话 / RPM）
- 供应商代理批量操作（批量删除、批量测试、批量质量检查）
- 供应商代理数据导入/导出
- 供应商代理过期预警通知
