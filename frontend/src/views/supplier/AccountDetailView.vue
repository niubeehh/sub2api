<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="account">
        <!-- 账号基本信息 -->
        <div class="rounded-lg border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800">
          <div class="flex items-center justify-between">
            <h2 class="text-xl font-semibold">{{ account.name }}</h2>
            <router-link to="/supplier/accounts" class="text-sm text-blue-600 hover:underline">返回列表</router-link>
          </div>
          <div class="mt-4 grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
            <div><span class="text-gray-500">平台：</span>{{ account.platform }}</div>
            <div><span class="text-gray-500">类型：</span>{{ account.type }}</div>
            <div><span class="text-gray-500">状态：</span>{{ account.status }}</div>
            <div><span class="text-gray-500">最后使用：</span>{{ account.last_used_at ? formatDate(account.last_used_at) : '-' }}</div>
          </div>
          <!-- 操作按钮 -->
          <div class="mt-4 flex flex-wrap gap-2">
            <button class="rounded-md bg-cyan-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-cyan-700" @click="refreshAccount">刷新凭据</button>
            <button v-if="account.status === 'error'" class="rounded-md bg-orange-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-orange-700" @click="clearError">清除错误</button>
            <button class="rounded-md bg-yellow-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-yellow-700" @click="clearRateLimit">清除限流</button>
            <button class="rounded-md bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700" @click="deleteAccount">删除账号</button>
          </div>
        </div>

        <!-- 今日统计 -->
        <div class="rounded-lg border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800">
          <h3 class="mb-4 text-lg font-medium">今日统计</h3>
          <div v-if="todayStats" class="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div>
              <div class="text-sm text-gray-500">请求数</div>
              <div class="mt-1 text-xl font-medium">{{ formatNumber(todayStats.request_count ?? todayStats.total_requests ?? 0) }}</div>
            </div>
            <div>
              <div class="text-sm text-gray-500">消耗</div>
              <div class="mt-1 text-xl font-medium">${{ formatCost(todayStats.cost ?? todayStats.total_cost ?? 0) }}</div>
            </div>
            <div>
              <div class="text-sm text-gray-500">Tokens</div>
              <div class="mt-1 text-xl font-medium">{{ formatNumber(todayStats.total_tokens ?? 0) }}</div>
            </div>
          </div>
          <div v-else class="text-sm text-gray-500">暂无今日数据</div>
        </div>

        <!-- 历史统计 -->
        <div class="rounded-lg border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800">
          <div class="mb-4 flex items-center justify-between">
            <h3 class="text-lg font-medium">历史统计</h3>
            <select v-model="statsDays" class="rounded-md border border-gray-300 px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-700" @change="loadStats">
              <option :value="7">7 天</option>
              <option :value="30">30 天</option>
              <option :value="90">90 天</option>
            </select>
          </div>
          <div v-if="stats" class="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div>
              <div class="text-sm text-gray-500">总请求</div>
              <div class="mt-1 text-xl font-medium">{{ formatNumber(stats.total_requests ?? 0) }}</div>
            </div>
            <div>
              <div class="text-sm text-gray-500">总消耗</div>
              <div class="mt-1 text-xl font-medium">${{ formatCost(stats.total_cost ?? 0) }}</div>
            </div>
            <div>
              <div class="text-sm text-gray-500">输入 Tokens</div>
              <div class="mt-1 text-xl font-medium">{{ formatNumber(stats.total_input_tokens ?? 0) }}</div>
            </div>
            <div>
              <div class="text-sm text-gray-500">输出 Tokens</div>
              <div class="mt-1 text-xl font-medium">{{ formatNumber(stats.total_output_tokens ?? 0) }}</div>
            </div>
          </div>
          <div v-else class="text-sm text-gray-500">暂无数据</div>
        </div>
      </template>
      <div v-else class="py-12 text-center text-gray-500">账号不存在</div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { supplierAPI, type SupplierAccount } from '@/api/supplier'

const route = useRoute()
const accountId = Number(route.params.id)

const loading = ref(false)
const account = ref<SupplierAccount | null>(null)
const todayStats = ref<any>(null)
const stats = ref<any>(null)
const statsDays = ref(30)

const formatNumber = (n: number) => n?.toLocaleString() ?? '0'
const formatCost = (n: number) => (n ?? 0).toFixed(4)
const formatDate = (s: string) => new Date(s).toLocaleString()

const loadAccount = async () => {
  try {
    account.value = await supplierAPI.getAccount(accountId)
  } catch (error) {
    console.error('Failed to load account:', error)
  }
}

const loadTodayStats = async () => {
  try {
    todayStats.value = await supplierAPI.getAccountTodayStats(accountId)
  } catch (error) {
    console.error('Failed to load today stats:', error)
  }
}

const loadStats = async () => {
  try {
    stats.value = await supplierAPI.getAccountStats(accountId, statsDays.value)
  } catch (error) {
    console.error('Failed to load stats:', error)
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([loadAccount(), loadTodayStats(), loadStats()])
  } finally {
    loading.value = false
  }
})

// 账号操作
const refreshAccount = async () => {
  try {
    account.value = await supplierAPI.refreshAccount(accountId)
  } catch (error) {
    console.error('Failed to refresh:', error)
  }
}

const clearError = async () => {
  try {
    account.value = await supplierAPI.clearAccountError(accountId)
  } catch (error) {
    console.error('Failed to clear error:', error)
  }
}

const clearRateLimit = async () => {
  try {
    account.value = await supplierAPI.clearAccountRateLimit(accountId)
  } catch (error) {
    console.error('Failed to clear rate limit:', error)
  }
}

const deleteAccount = async () => {
  if (!confirm(`确定删除账号「${account.value?.name}」吗？此操作不可撤销。`)) return
  try {
    await supplierAPI.deleteAccount(accountId)
    window.location.href = '/supplier/accounts'
  } catch (error) {
    console.error('Failed to delete:', error)
  }
}
</script>
