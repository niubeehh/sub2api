<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="dashboard">
        <!-- Row 1: Core Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Account Count -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
                <Icon name="server" size="md" class="text-purple-600 dark:text-purple-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">账号数量</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ dashboard.account_count }}</p>
              </div>
            </div>
          </div>

          <!-- Total Requests -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
                <Icon name="chart" size="md" class="text-green-600 dark:text-green-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">总请求数</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatNumber(dashboard.total_requests) }}</p>
              </div>
            </div>
          </div>

          <!-- Total Cost -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
                <Icon name="dollar" size="md" class="text-blue-600 dark:text-blue-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">总消耗 (USD)</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">${{ formatCost(dashboard.total_cost) }}</p>
              </div>
            </div>
          </div>

          <!-- Actual Cost -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
                <Icon name="trendingUp" size="md" class="text-emerald-600 dark:text-emerald-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">实际成本 (USD)</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">${{ formatCost(dashboard.total_actual_cost) }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Row 2: Token Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Input Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">输入 Tokens</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatTokens(dashboard.total_input_tokens) }}</p>
              </div>
            </div>
          </div>

          <!-- Output Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-indigo-100 p-2 dark:bg-indigo-900/30">
                <Icon name="database" size="md" class="text-indigo-600 dark:text-indigo-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">输出 Tokens</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatTokens(dashboard.total_output_tokens) }}</p>
              </div>
            </div>
          </div>

          <!-- Total Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-violet-100 p-2 dark:bg-violet-900/30">
                <Icon name="bolt" size="md" class="text-violet-600 dark:text-violet-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">总 Tokens</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatTokens(dashboard.total_tokens) }}</p>
              </div>
            </div>
          </div>

          <!-- Avg Duration -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-rose-100 p-2 dark:bg-rose-900/30">
                <Icon name="clock" size="md" class="text-rose-600 dark:text-rose-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">平均耗时</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatDuration(dashboard.average_duration_ms) }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Row 3: Charts -->
        <div class="space-y-4">
          <!-- Date Range Filter -->
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
              <button @click="loadCharts" :disabled="loadingCharts" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.granularity') }}:</span>
                <div class="w-28">
                  <Select v-model="granularity" :options="granularityOptions" @change="loadCharts" />
                </div>
              </div>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <ModelDistributionChart
              v-model:metric="modelDistributionMetric"
              :model-stats="modelStats"
              :loading="loadingCharts"
              :show-source-toggle="false"
              :show-metric-toggle="true"
              :enable-breakdown="false"
              :show-account-cost="true"
              :start-date="startDate"
              :end-date="endDate"
            />
            <TokenUsageTrend :trend-data="trendData" :loading="loadingCharts" />
          </div>
        </div>

        <!-- Row 4: Account Usage Detail + Quick Actions -->
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <!-- Account Usage Detail -->
          <div v-if="accountUsage.length > 0" class="card lg:col-span-2">
            <div class="flex items-center justify-between border-b border-gray-200 p-4 dark:border-gray-700">
              <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">账号消耗明细</h3>
              <router-link to="/supplier/accounts" class="text-xs font-medium text-blue-600 hover:underline dark:text-blue-400">
                查看全部 →
              </router-link>
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">账号名称</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">平台</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">今日消耗</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">今日请求</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">状态</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
                  <tr v-for="item in accountUsage" :key="item.id" class="hover:bg-gray-50 dark:hover:bg-gray-800">
                    <td class="px-4 py-3 text-sm">
                      <router-link :to="`/supplier/accounts/${item.id}`" class="font-medium text-blue-600 hover:underline dark:text-blue-400">{{ item.name }}</router-link>
                    </td>
                    <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-400">{{ item.platform }}</td>
                    <td class="px-4 py-3 text-right text-sm font-medium text-gray-900 dark:text-white">${{ formatCost(item.today_cost) }}</td>
                    <td class="px-4 py-3 text-right text-sm font-medium text-gray-900 dark:text-white">{{ formatNumber(item.today_requests) }}</td>
                    <td class="px-4 py-3 text-sm">
                      <span :class="statusBadgeClass(item.status)">{{ item.status }}</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Quick Actions -->
          <div class="card lg:col-span-1">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">快捷操作</h2>
            </div>
            <div class="space-y-3 p-4">
              <button @click="router.push('/supplier/accounts')" class="group flex w-full items-center gap-4 rounded-xl bg-gray-50 p-4 text-left transition-all duration-200 hover:bg-gray-100 dark:bg-dark-800/50 dark:hover:bg-dark-800">
                <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-primary-100 transition-transform group-hover:scale-105 dark:bg-primary-900/30">
                  <Icon name="server" size="lg" class="text-primary-600 dark:text-primary-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">账号管理</p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">管理上游账号</p>
                </div>
                <Icon name="chevronRight" size="md" class="text-gray-400 transition-colors group-hover:text-primary-500 dark:text-dark-500" />
              </button>

              <button @click="router.push('/supplier/usage')" class="group flex w-full items-center gap-4 rounded-xl bg-gray-50 p-4 text-left transition-all duration-200 hover:bg-gray-100 dark:bg-dark-800/50 dark:hover:bg-dark-800">
                <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-100 transition-transform group-hover:scale-105 dark:bg-emerald-900/30">
                  <Icon name="chart" size="lg" class="text-emerald-600 dark:text-emerald-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">用量明细</p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">查看详细用量日志</p>
                </div>
                <Icon name="chevronRight" size="md" class="text-gray-400 transition-colors group-hover:text-emerald-500 dark:text-dark-500" />
              </button>

              <button @click="router.push('/supplier/proxies')" class="group flex w-full items-center gap-4 rounded-xl bg-gray-50 p-4 text-left transition-all duration-200 hover:bg-gray-100 dark:bg-dark-800/50 dark:hover:bg-dark-800">
                <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-sky-100 transition-transform group-hover:scale-105 dark:bg-sky-900/30">
                  <Icon name="globe" size="lg" class="text-sky-600 dark:text-sky-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">代理管理</p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">管理代理配置</p>
                </div>
                <Icon name="chevronRight" size="md" class="text-gray-400 transition-colors group-hover:text-sky-500 dark:text-dark-500" />
              </button>
            </div>
          </div>
        </div>
      </template>

      <div v-else class="py-12 text-center text-gray-500 dark:text-gray-400">暂无数据</div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import Icon from '@/components/icons/Icon.vue'
import { supplierAPI, type SupplierDashboardResponse } from '@/api/supplier'
import { formatDateLocalInput } from '@/utils/format'
import type { TrendDataPoint, ModelStat } from '@/types'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const loadingCharts = ref(false)
const dashboard = ref<SupplierDashboardResponse | null>(null)
const accountUsage = ref<any[]>([])

// 图表状态
const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateLocalInput(new Date()))
const granularity = ref<'day' | 'hour'>('day')
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const modelDistributionMetric = ref<'tokens' | 'actual_cost'>('tokens')

const granularityOptions = computed<SelectOption[]>(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') },
])

const toFiniteNumber = (value: unknown): number => {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

const formatNumber = (value: number | null | undefined): string => {
  return toFiniteNumber(value).toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  const v = toFiniteNumber(value)
  if (v >= 1000) return (v / 1000).toFixed(2) + 'K'
  if (v >= 1) return v.toFixed(2)
  if (v >= 0.01) return v.toFixed(3)
  return v.toFixed(4)
}

const formatTokens = (value: number | null | undefined): string => {
  const v = toFiniteNumber(value)
  if (v >= 1_000_000_000) return (v / 1_000_000_000).toFixed(2) + 'B'
  if (v >= 1_000_000) return (v / 1_000_000).toFixed(2) + 'M'
  if (v >= 1_000) return (v / 1_000).toFixed(2) + 'K'
  return v.toLocaleString()
}

const formatDuration = (ms: number | null | undefined): string => {
  const v = toFiniteNumber(ms)
  if (v >= 1000) return (v / 1000).toFixed(2) + 's'
  return Math.round(v) + 'ms'
}

function statusBadgeClass(status: string) {
  return status === 'active'
    ? 'inline-flex items-center rounded-md bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700 ring-1 ring-green-200 dark:bg-green-900/30 dark:text-green-400 dark:ring-green-800'
    : 'inline-flex items-center rounded-md bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:ring-gray-700'
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  loadCharts()
}

const loadCharts = async () => {
  loadingCharts.value = true
  try {
    const [snapshot, modelRes] = await Promise.all([
      supplierAPI.getDashboardSnapshotV2({
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value,
        include_trend: true,
        include_group_stats: false,
      }),
      supplierAPI.getDashboardModels({
        start_date: startDate.value,
        end_date: endDate.value,
        model_source: 'requested',
      }),
    ])
    trendData.value = snapshot.trend || []
    modelStats.value = modelRes.models || []
  } catch (error) {
    console.error('Failed to load dashboard charts:', error)
    trendData.value = []
    modelStats.value = []
  } finally {
    loadingCharts.value = false
  }
}

const load = async () => {
  loading.value = true
  try {
    dashboard.value = await supplierAPI.getDashboard(30)

    const accountsRes = await supplierAPI.listAccounts({ page: 1, page_size: 100 })
    const accounts = accountsRes.items || []
    if (accounts.length > 0) {
      const accountIds = accounts.map((a: any) => a.id)
      try {
        const todayStats = await supplierAPI.getBatchTodayStats(accountIds)

        accountUsage.value = accounts.map((a: any) => ({
          id: a.id,
          name: a.name,
          platform: a.platform,
          status: a.status,
          today_cost: todayStats?.[a.id]?.cost || 0,
          today_requests: todayStats?.[a.id]?.requests || 0,
        }))
      } catch {
        accountUsage.value = accounts.map((a: any) => ({
          id: a.id,
          name: a.name,
          platform: a.platform,
          status: a.status,
          today_cost: 0,
          today_requests: 0,
        }))
      }
    }
  } catch (error) {
    console.error('Failed to load supplier dashboard:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  loadCharts()
})
</script>
