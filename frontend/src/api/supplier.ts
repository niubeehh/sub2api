/**
 * Supplier API Client
 * 供应商面板专用 API：账号列表、消耗统计、用量明细
 */
import { apiClient } from './client'

export interface SupplierAccount {
  id: number
  name: string
  platform: string
  type: string
  status: string
  owner_id?: number | null
  created_at?: string
  updated_at?: string
  last_used_at?: string | null
  expires_at?: string | null
  priority?: number
  concurrency?: number
  schedulable?: boolean
  notes?: string | null
}

export interface SupplierAccountListResponse {
  items: SupplierAccount[]
  total: number
  page: number
  page_size: number
}

export interface SupplierDashboardResponse {
  total_requests: number
  total_cost: number
  total_actual_cost: number
  total_input_tokens: number
  total_output_tokens: number
  total_tokens: number
  average_duration_ms: number
  account_count: number
  start_time: string
  end_time: string
}

export interface SupplierUsageListResponse {
  items: any[]
  total: number
  page: number
  page_size: number
}

export interface SupplierUsageLogFilters {
  account_id?: number
  model?: string
  billing_mode?: string
  request_type?: string
  start_date?: string
  end_date?: string
  timezone?: string
  sort_by?: string
  sort_order?: string
  page?: number
  page_size?: number
}

export const supplierAPI = {
  /** 供应商仪表盘：整体消耗汇总 */
  async getDashboard(days?: number): Promise<SupplierDashboardResponse> {
    const params = days ? { days } : {}
    const res = await apiClient.get('/supplier/dashboard', { params })
    return res.data
  },

  /** 供应商仪表盘模型分布 */
  async getDashboardModels(params?: {
    start_date?: string
    end_date?: string
    account_id?: number
    model?: string
    model_source?: string
    request_type?: string
    billing_type?: number | null
    billing_mode?: string | null
    timezone?: string
  }): Promise<{ models: any[]; start_date: string; end_date: string }> {
    const res = await apiClient.get('/supplier/dashboard/models', { params })
    return res.data
  },

  /** 供应商仪表盘快照（趋势 + 模型 + 分组） */
  async getDashboardSnapshotV2(params?: {
    start_date?: string
    end_date?: string
    granularity?: 'day' | 'hour'
    account_id?: number
    model?: string
    request_type?: string
    billing_type?: number | null
    billing_mode?: string | null
    include_trend?: boolean
    include_model_stats?: boolean
    include_group_stats?: boolean
    timezone?: string
  }): Promise<{
    generated_at: string
    start_date: string
    end_date: string
    granularity: string
    trend?: any[]
    models?: any[]
    groups?: any[]
  }> {
    const res = await apiClient.get('/supplier/dashboard/snapshot-v2', { params })
    return res.data
  },

  /** 供应商账号列表 */
  async listAccounts(params?: {
    platform?: string
    type?: string
    status?: string
    search?: string
    group?: string
    page?: number
    page_size?: number
    sort_by?: string
    sort_order?: string
  }): Promise<SupplierAccountListResponse> {
    const res = await apiClient.get('/supplier/accounts', { params })
    return res.data
  },

  /** 供应商账号详情 */
  async getAccount(id: number): Promise<SupplierAccount> {
    const res = await apiClient.get(`/supplier/accounts/${id}`)
    return res.data
  },

  /** 单账号统计 */
  async getAccountStats(id: number, days?: number): Promise<any> {
    const params = days ? { days } : {}
    const res = await apiClient.get(`/supplier/accounts/${id}/stats`, { params })
    return res.data
  },

  /** 单账号用量 */
  async getAccountUsage(id: number, source?: string, force?: boolean): Promise<any> {
    const params: any = {}
    if (source) params.source = source
    if (force) params.force = 'true'
    const res = await apiClient.get(`/supplier/accounts/${id}/usage`, { params })
    return res.data
  },

  /** 单账号今日统计 */
  async getAccountTodayStats(id: number): Promise<any> {
    const res = await apiClient.get(`/supplier/accounts/${id}/today-stats`)
    return res.data
  },

  /** 用量明细列表 */
  async listUsage(filters?: SupplierUsageLogFilters): Promise<SupplierUsageListResponse> {
    const res = await apiClient.get('/supplier/usage', { params: filters })
    return res.data
  },

  // ==================== 阶段二：写操作 ====================

  /** 创建账号（owner_id 由后端强制注入） */
  async createAccount(data: {
    name: string
    platform: string
    type: string
    credentials: Record<string, any>
    extra?: Record<string, any>
    proxy_id?: number | null
    concurrency?: number
    priority?: number
    rate_multiplier?: number | null
    load_factor?: number | null
    group_ids?: number[]
    expires_at?: number | null
    auto_pause_on_expired?: boolean | null
    upstream_billing_probe_enabled?: boolean | null
    confirm_mixed_channel_risk?: boolean | null
  }): Promise<SupplierAccount> {
    const res = await apiClient.post('/supplier/accounts', data)
    return res.data
  },

  /** 编辑账号 */
  async updateAccount(id: number, data: Partial<{
    name: string
    notes: string | null
    type: string
    credentials: Record<string, any>
    extra: Record<string, any>
    proxy_id: number | null
    concurrency: number
    priority: number
    rate_multiplier: number | null
    load_factor: number | null
    status: string
    group_ids: number[]
    expires_at: number | null
    auto_pause_on_expired: boolean | null
    upstream_billing_probe_enabled: boolean | null
    upstream_billing_rate_sync_enabled: boolean | null
    confirm_mixed_channel_risk: boolean | null
  }>): Promise<SupplierAccount> {
    const res = await apiClient.put(`/supplier/accounts/${id}`, data)
    return res.data
  },

  /** 删除账号 */
  async deleteAccount(id: number): Promise<void> {
    await apiClient.delete(`/supplier/accounts/${id}`)
  },

  /** 测试账号连接（SSE 流，返回 EventSource 或 fetch 流） */
  async testAccount(id: number, data: { model_id?: string; prompt?: string; mode?: string; image_data_url?: string; audio_data_url?: string }): Promise<Response> {
    return apiClient.post(`/supplier/accounts/${id}/test`, data, { responseType: 'stream' })
  },

  /** 刷新账号凭据 */
  async refreshAccount(id: number): Promise<SupplierAccount> {
    const res = await apiClient.post(`/supplier/accounts/${id}/refresh`)
    return res.data
  },

  /** 清除账号错误状态 */
  async clearAccountError(id: number): Promise<SupplierAccount> {
    const res = await apiClient.post(`/supplier/accounts/${id}/clear-error`)
    return res.data
  },

  /** 清除账号限流状态 */
  async clearAccountRateLimit(id: number): Promise<SupplierAccount> {
    const res = await apiClient.post(`/supplier/accounts/${id}/clear-rate-limit`)
    return res.data
  },

  // ==================== OAuth 流程 ====================

  /** 生成 Claude OAuth 授权 URL */
  async generateAuthURL(proxy_id?: number | null): Promise<{ url: string; session_id: string }> {
    const res = await apiClient.post('/supplier/accounts/generate-auth-url', { proxy_id })
    return res.data
  },

  /** 生成 setup token 授权 URL */
  async generateSetupTokenURL(proxy_id?: number | null): Promise<{ url: string; session_id: string }> {
    const res = await apiClient.post('/supplier/accounts/generate-setup-token-url', { proxy_id })
    return res.data
  },

  /** 交换授权码 */
  async exchangeCode(data: { session_id: string; code: string; proxy_id?: number | null }): Promise<any> {
    const res = await apiClient.post('/supplier/accounts/exchange-code', data)
    return res.data
  },

  /** 交换 setup token 授权码 */
  async exchangeSetupTokenCode(data: { session_id: string; code: string; proxy_id?: number | null }): Promise<any> {
    const res = await apiClient.post('/supplier/accounts/exchange-setup-token-code', data)
    return res.data
  },

  /** Cookie 认证 */
  async cookieAuth(data: { code: string; proxy_id?: number | null }): Promise<any> {
    const res = await apiClient.post('/supplier/accounts/cookie-auth', data)
    return res.data
  },

  /** Setup token cookie 认证 */
  async setupTokenCookieAuth(data: { code: string; proxy_id?: number | null }): Promise<any> {
    const res = await apiClient.post('/supplier/accounts/setup-token-cookie-auth', data)
    return res.data
  },

  // ==================== Codex Session 导入 ====================

  /** 导入 Codex session */
  async importCodexSession(data: {
    content?: string
    contents?: string[]
    name?: string
    notes?: string | null
    group_ids?: number[]
    proxy_id?: number | null
    concurrency?: number
    priority?: number
    rate_multiplier?: number | null
    load_factor?: number | null
    expires_at?: number | null
    auto_pause_on_expired?: boolean | null
    credential_extras?: Record<string, any>
    extra?: Record<string, any>
    update_existing?: boolean | null
    skip_default_group_bind?: boolean | null
    confirm_mixed_channel_risk?: boolean | null
  }): Promise<{
    total: number
    created: number
    updated: number
    skipped: number
    failed: number
    items: any[]
    warnings?: any[]
    errors?: any[]
  }> {
    const res = await apiClient.post('/supplier/accounts/import/codex-session', data)
    return res.data
  },

  // ==================== 阶段三：使用记录增强 ====================

  /** 使用记录统计 */
  async getUsageStats(filters?: {
    account_id?: number
    model?: string
    start_date?: string
    end_date?: string
    timezone?: string
  }): Promise<any> {
    const res = await apiClient.get('/supplier/usage/stats', { params: filters })
    return res.data
  },

  /** 批量查询账号用量 */
  async getBatchUsage(account_ids: number[], force?: boolean): Promise<{
    usage: Record<number, any>
    errors: Record<number, string>
  }> {
    const res = await apiClient.post('/supplier/accounts/usage/batch', { account_ids, force })
    return res.data
  },

  /** 批量查询账号今日统计 */
  async getBatchTodayStats(account_ids: number[]): Promise<Record<number, any>> {
    const res = await apiClient.post('/supplier/accounts/today-stats/batch', { account_ids })
    return res.data
  },

  // ==================== 阶段三：代理管理 ====================

  /** 供应商代理列表 */
  async listProxies(params?: {
    protocol?: string
    status?: string
    search?: string
    sort_by?: string
    sort_order?: string
    page?: number
    page_size?: number
  }): Promise<any> {
    const res = await apiClient.get('/supplier/proxies', { params })
    return res.data
  },

  /** 获取所有活跃代理 */
  async getAllProxies(): Promise<any[]> {
    const res = await apiClient.get('/supplier/proxies/all')
    return res.data
  },

  /** 获取代理详情 */
  async getProxy(id: number): Promise<any> {
    const res = await apiClient.get(`/supplier/proxies/${id}`)
    return res.data
  },

  /** 创建代理 */
  async createProxy(data: {
    name: string
    protocol: string
    host: string
    port: number
    username?: string
    password?: string
    expires_at?: number | null
    fallback_mode?: string
    backup_proxy_id?: number | null
    expiry_warn_days?: number
  }): Promise<any> {
    const res = await apiClient.post('/supplier/proxies', data)
    return res.data
  },

  /** 更新代理 */
  async updateProxy(id: number, data: Partial<{
    name: string
    protocol: string
    host: string
    port: number
    username: string
    password: string
    status: string
    expires_at: number | null
    fallback_mode: string
    backup_proxy_id: number | null
    expiry_warn_days: number
  }>): Promise<any> {
    const res = await apiClient.put(`/supplier/proxies/${id}`, data)
    return res.data
  },

  /** 删除代理 */
  async deleteProxy(id: number): Promise<void> {
    await apiClient.delete(`/supplier/proxies/${id}`)
  },

  /** 测试代理 */
  async testProxy(id: number): Promise<any> {
    const res = await apiClient.post(`/supplier/proxies/${id}/test`)
    return res.data
  },

  /** 代理质量检查 */
  async checkProxyQuality(id: number): Promise<any> {
    const res = await apiClient.post(`/supplier/proxies/${id}/quality-check`)
    return res.data
  },

  /** 获取使用此代理的账号 */
  async getProxyAccounts(id: number): Promise<any[]> {
    const res = await apiClient.get(`/supplier/proxies/${id}/accounts`)
    return res.data
  },
}
