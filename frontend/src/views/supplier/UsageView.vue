<template>
  <AppLayout>
    <div class="space-y-6">
      <UsageStatsCards :stats="usageStats" :show-account-cost="true" :strike-standard-cost="false" />

      <div class="space-y-4">
        <div class="card p-4">
          <div class="flex flex-wrap items-center gap-4">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.timeRange') }}:</span>
              <DateRangePicker
                v-model:start-date="startDate"
                v-model:end-date="endDate"
                @change="onDateRangeChange"
              />
            </div>
            <div class="ml-auto flex items-center gap-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.granularity') }}:</span>
              <div class="w-28">
                <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
              </div>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ModelDistributionChart
            v-model:metric="modelDistributionMetric"
            :model-stats="requestedModelStats"
            :loading="modelStatsLoading"
            :show-source-toggle="false"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :show-account-cost="true"
            :start-date="startDate"
            :end-date="endDate"
          />
          <GroupDistributionChart
            v-model:metric="groupDistributionMetric"
            :group-stats="groupStats"
            :loading="chartsLoading"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :show-account-cost="true"
            :start-date="startDate"
            :end-date="endDate"
          />
        </div>

        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <EndpointDistributionChart
            v-model:source="endpointDistributionSource"
            v-model:metric="endpointDistributionMetric"
            :endpoint-stats="inboundEndpointStats"
            :upstream-endpoint-stats="upstreamEndpointStats"
            :endpoint-path-stats="endpointPathStats"
            :loading="endpointStatsLoading"
            :show-source-toggle="false"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :title="t('usage.endpointDistribution')"
            :start-date="startDate"
            :end-date="endDate"
          />
          <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
        </div>
      </div>

      <div class="card p-6">
        <div class="flex flex-wrap items-end justify-between gap-4">
          <div class="flex flex-1 flex-wrap items-end gap-4">
            <div class="w-full sm:w-auto sm:min-w-[220px]">
              <label class="input-label">账号</label>
              <Select v-model="filters.account_id" :options="accountOptions" searchable clearable @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[220px]">
              <label class="input-label">{{ t('usage.model') }}</label>
              <Select v-model="filters.model" :options="modelOptions" searchable clearable @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[180px]">
              <label class="input-label">{{ t('usage.type') }}</label>
              <Select v-model="filters.request_type" :options="requestTypeOptions" @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[200px]">
              <label class="input-label">{{ t('admin.usage.billingMode') }}</label>
              <Select v-model="filters.billing_mode" :options="billingModeOptions" @change="applyFilters" />
            </div>
          </div>

          <div class="flex w-full flex-wrap items-center justify-end gap-3 sm:w-auto">
            <button type="button" @click="refreshData" :disabled="loading" class="btn btn-secondary">
              {{ t('common.refresh') }}
            </button>
            <button type="button" @click="resetFilters" class="btn btn-secondary">
              {{ t('common.reset') }}
            </button>
            <div class="relative" ref="columnDropdownRef">
              <button
                type="button"
                @click="showColumnDropdown = !showColumnDropdown"
                class="btn btn-secondary px-2 md:px-3"
                :title="t('admin.users.columnSettings')"
              >
                <Icon name="grid" size="sm" />
                <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
              </button>
              <div
                v-if="showColumnDropdown"
                class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
              >
                <button
                  v-for="col in toggleableColumns"
                  :key="col.key"
                  type="button"
                  @click="toggleColumn(col.key)"
                  class="flex w-full items-center justify-between px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
                >
                  <span>{{ col.label }}</span>
                  <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" class="text-primary-500" />
                </button>
              </div>
            </div>
            <button type="button" @click="exportToCSV" :disabled="exporting" class="btn btn-primary">
              {{ exporting ? t('usage.exporting') : t('usage.exportCsv') }}
            </button>
          </div>
        </div>
      </div>

      <UsageTable
        :data="usageLogs"
        :loading="loading"
        :columns="visibleColumns"
        :server-side-sort="true"
        :show-account-billing="true"
        :show-upstream-endpoint="true"
        default-sort-key="created_at"
        default-sort-order="desc"
        @sort="handleSort"
        @ipGeoBatchFailed="handleIpGeoBatchFailed"
      />

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { supplierAPI } from '@/api/supplier'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageStatsCards from '@/components/admin/usage/UsageStatsCards.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatReasoningEffort } from '@/utils/format'
import { getBillingModeLabel, getDisplayBillingMode as resolveDisplayBillingMode } from '@/utils/billingMode'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import type {
  EndpointStat,
  GroupStat,
  ModelStat,
  TrendDataPoint,
  UsageRequestType,
  UsageStatsResponse,
} from '@/types'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const appStore = useAppStore()

type DistributionMetric = 'tokens' | 'actual_cost'
type EndpointSource = 'inbound' | 'upstream' | 'path'

const usageStats = ref<UsageStatsResponse | null>(null)
const usageLogs = ref<any[]>([])
const trendData = ref<TrendDataPoint[]>([])
const requestedModelStats = ref<ModelStat[]>([])
const groupStats = ref<GroupStat[]>([])
const inboundEndpointStats = ref<EndpointStat[]>([])
const upstreamEndpointStats = ref<EndpointStat[]>([])
const endpointPathStats = ref<EndpointStat[]>([])

const loading = ref(false)
const chartsLoading = ref(false)
const modelStatsLoading = ref(false)
const endpointStatsLoading = ref(false)
const exporting = ref(false)

let abortController: AbortController | null = null
let chartReqSeq = 0
let statsReqSeq = 0
let modelStatsReqSeq = 0

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const getLast24HoursRangeDates = () => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const getGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  return Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24)) <= 1 ? 'hour' : 'day'
}

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const granularity = ref<'day' | 'hour'>(getGranularityForRange(startDate.value, endDate.value))

const modelDistributionMetric = ref<DistributionMetric>('tokens')
const groupDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionSource = ref<EndpointSource>('inbound')

const filters = ref<{
  account_id: number | null
  model: string | null
  request_type: UsageRequestType | null
  billing_mode: string | null
}>({
  account_id: null,
  model: null,
  request_type: null,
  billing_mode: null,
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc',
})

const granularityOptions = computed<SelectOption[]>(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') },
])
const requestTypeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
])
const billingModeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allBillingModes') },
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') },
])

// 账号选项
const accounts = ref<{ id: number; name: string }[]>([])
const accountOptions = computed<SelectOption[]>(() => [
  { value: null, label: '全部账号' },
  ...accounts.value.map((a) => ({ value: a.id, label: a.name })),
])

// 模型选项（从 modelStats 动态填充）
const modelOptionValues = ref<string[]>([])
const modelOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allModels') },
  ...modelOptionValues.value.map((m) => ({ value: m, label: m })),
])

const normalizedFilters = computed(() => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : undefined
  return {
    account_id: filters.value.account_id ?? undefined,
    model: filters.value.model || undefined,
    request_type: requestType ?? undefined,
    billing_mode: filters.value.billing_mode ?? undefined,
    stream: legacyStream === null ? undefined : legacyStream,
    start_date: startDate.value,
    end_date: endDate.value,
  }
})

const buildUsageListParams = (page: number, pageSize: number) => ({
  page,
  page_size: pageSize,
  ...normalizedFilters.value,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order,
})

const loadLogs = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  loading.value = true
  try {
    const res = await supplierAPI.listUsage(buildUsageListParams(pagination.page, pagination.page_size))
    if (!controller.signal.aborted) {
      usageLogs.value = res.items || []
      pagination.total = res.total || 0
    }
  } catch (error: any) {
    if (error?.name !== 'AbortError' && error?.code !== 'ERR_CANCELED') {
      appStore.showError(t('usage.failedToLoad'))
    }
  } finally {
    if (abortController === controller) loading.value = false
  }
}

const loadStats = async () => {
  const seq = ++statsReqSeq
  endpointStatsLoading.value = true
  try {
    const stats = await supplierAPI.getUsageStats(normalizedFilters.value)
    if (seq !== statsReqSeq) return
    usageStats.value = stats
    inboundEndpointStats.value = stats.endpoints || []
    upstreamEndpointStats.value = stats.upstream_endpoints || []
    endpointPathStats.value = stats.endpoint_paths || []
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
    inboundEndpointStats.value = []
    upstreamEndpointStats.value = []
    endpointPathStats.value = []
  } finally {
    if (seq === statsReqSeq) endpointStatsLoading.value = false
  }
}

const loadModelStats = async () => {
  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  try {
    const response = await supplierAPI.getDashboardModels({
      ...normalizedFilters.value,
      model_source: 'requested',
    })
    if (seq !== modelStatsReqSeq) return
    requestedModelStats.value = response.models || []
    refreshModelOptions(response.models || [])
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load model stats:', error)
    requestedModelStats.value = []
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  try {
    const snapshot = await supplierAPI.getDashboardSnapshotV2({
      ...normalizedFilters.value,
      granularity: granularity.value,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: true,
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
    groupStats.value = snapshot.groups || []
  } catch (error) {
    if (seq !== chartReqSeq) return
    console.error('Failed to load chart data:', error)
    trendData.value = []
    groupStats.value = []
  } finally {
    if (seq === chartReqSeq) chartsLoading.value = false
  }
}

const refreshModelOptions = (models: ModelStat[]) => {
  const current = filters.value.model
  const set = new Set(modelOptionValues.value)
  models.forEach((item) => {
    if (item.model) set.add(item.model)
  })
  if (current) set.add(current)
  modelOptionValues.value = Array.from(set).sort()
}

const applyFilters = () => {
  pagination.page = 1
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
}

const refreshData = () => {
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
}

const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = {
    account_id: null,
    model: null,
    request_type: null,
    billing_mode: null,
  }
  granularity.value = getGranularityForRange(range.start, range.end)
  applyFilters()
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  applyFilters()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void loadLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadLogs()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadLogs()
}

const handleIpGeoBatchFailed = () => {
  appStore.showError(t('usage.ipGeo.batchFailed'))
}

// ==================== CSV 导出 ====================

const getRequestTypeExportText = (log: any): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return 'Cyber'
  if (requestType === 'live') return 'Live'
  if (requestType === 'ws_v2') return 'WS'
  if (requestType === 'stream') return 'Stream'
  if (requestType === 'sync') return 'Sync'
  return 'Unknown'
}

const escapeCSVValue = (value: unknown): string => {
  if (value == null) return ''
  const str = String(value)
  const escaped = str.replace(/"/g, '""')
  if (/^[=+\-@\t\r]/.test(str)) return `"\'${escaped}"`
  if (/[,"\n\r]/.test(str)) return `"${escaped}"`
  return str
}

const exportToCSV = async () => {
  if (pagination.total === 0) {
    appStore.showWarning(t('usage.noDataToExport'))
    return
  }
  exporting.value = true
  appStore.showInfo(t('usage.preparingExport'))
  try {
    const allLogs: any[] = []
    const pageSize = 100
    const totalPages = Math.ceil(pagination.total / pageSize)
    for (let page = 1; page <= totalPages; page++) {
      const response = await supplierAPI.listUsage(buildUsageListParams(page, pageSize))
      allLogs.push(...(response.items || []))
    }
    if (allLogs.length === 0) {
      appStore.showWarning(t('usage.noDataToExport'))
      return
    }
    const headers = [
      'Time',
      'Account',
      'Model',
      'Upstream Model',
      'Reasoning Effort',
      'Inbound Endpoint',
      'Upstream Endpoint',
      'IP Address',
      'Type',
      'Billing Mode',
      'Input Tokens',
      'Output Tokens',
      'Cache Read Tokens',
      'Cache Creation Tokens',
      'Rate Multiplier',
      'Account Billed',
      'Billed Cost',
      'Original Cost',
      'First Token (ms)',
      'Duration (ms)',
    ]
    const rows = allLogs.map((log) => [
      log.created_at,
      log.account?.name || '',
      log.model,
      log.upstream_model || '',
      formatReasoningEffort(log.reasoning_effort),
      log.inbound_endpoint || '',
      log.upstream_endpoint || '',
      log.ip_address || '',
      getRequestTypeExportText(log),
      getBillingModeLabel(resolveDisplayBillingMode(log), t),
      log.input_tokens,
      log.output_tokens,
      log.cache_read_tokens,
      log.cache_creation_tokens,
      log.rate_multiplier,
      ((log.account_stats_cost ?? log.total_cost) * (log.account_rate_multiplier ?? 1)).toFixed(6),
      log.actual_cost?.toFixed(8) || '0',
      log.total_cost?.toFixed(8) || '0',
      log.first_token_ms ?? '',
      log.duration_ms ?? '',
    ].map(escapeCSVValue))
    const csvContent = [
      headers.map(escapeCSVValue).join(','),
      ...rows.map((row) => row.join(',')),
    ].join('\n')
    const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `supplier_usage_${startDate.value}_to_${endDate.value}.csv`
    link.click()
    window.URL.revokeObjectURL(url)
    appStore.showSuccess(t('usage.exportSuccess'))
  } catch (error) {
    console.error('CSV Export failed:', error)
    appStore.showError(t('usage.exportFailed'))
  } finally {
    exporting.value = false
  }
}

// ==================== 列设置 ====================

const ALWAYS_VISIBLE = ['created_at']
const DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const HIDDEN_COLUMNS_KEY = 'supplier-usage-hidden-columns'

const allColumns = computed<Column[]>(() => [
  { key: 'account', label: '账号', sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'ip_address', label: 'IP', sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'request_id', label: t('admin.usage.requestId'), sortable: false },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
])

const hiddenColumns = reactive<Set<string>>(new Set())
const toggleableColumns = computed(() => allColumns.value.filter((col) => !ALWAYS_VISIBLE.includes(col.key)))
const visibleColumns = computed(() =>
  allColumns.value.filter((col) => ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key))
)
const isColumnVisible = (key: string) => !hiddenColumns.has(key)
const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) hiddenColumns.delete(key)
  else hiddenColumns.add(key)
  localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
}
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    const values = saved ? JSON.parse(saved) as string[] : DEFAULT_HIDDEN_COLUMNS
    values.forEach((key) => hiddenColumns.add(key))
  } catch {
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
  }
}

const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)
const handleColumnClickOutside = (event: MouseEvent) => {
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(event.target as HTMLElement)) {
    showColumnDropdown.value = false
  }
}

// ==================== 初始化 ====================

const loadAccountOptions = async () => {
  try {
    const res = await supplierAPI.listAccounts({ page: 1, page_size: 100 })
    accounts.value = (res.items || []).map((a: any) => ({ id: a.id, name: a.name }))
  } catch (error) {
    console.error('Failed to load account options:', error)
  }
}

onMounted(() => {
  loadSavedColumns()
  document.addEventListener('click', handleColumnClickOutside)
  void loadAccountOptions()
  refreshData()
})

onUnmounted(() => {
  abortController?.abort()
  document.removeEventListener('click', handleColumnClickOutside)
})
</script>
