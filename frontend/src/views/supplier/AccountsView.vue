<template>
  <AppLayout>
    <TablePageLayout>
      <!-- Actions -->
      <template #actions>
        <div class="flex items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <button class="btn btn-primary" @click="showCreateModal = true">
              <Icon name="plus" size="sm" class="mr-1.5" />
              创建账号
            </button>
            <button class="btn btn-secondary" @click="showImportModal = true">
              <Icon name="upload" size="sm" class="mr-1.5" />
              导入 Codex
            </button>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="loadAccounts">
            <Icon name="refresh" size="sm" class="mr-1.5" :class="{ 'animate-spin': loading }" />
            刷新
          </button>
        </div>
      </template>

      <!-- Filters -->
      <template #filters>
        <div class="card flex flex-wrap items-center gap-3 p-3">
          <input
            v-model="searchInput"
            type="text"
            placeholder="搜索账号名称..."
            class="input w-full sm:w-64"
            @keyup.enter="handleSearch"
          />
          <Select
            v-model="platformFilter"
            :options="platformOptions"
            class="w-full sm:w-40"
            @change="loadAccounts"
          />
          <Select
            v-model="statusFilter"
            :options="statusOptions"
            class="w-full sm:w-32"
            @change="loadAccounts"
          />
          <button class="btn btn-secondary" @click="handleSearch">查询</button>
        </div>
      </template>

      <!-- Table -->
      <template #table>
        <DataTable
          :columns="columns"
          :data="accounts"
          :loading="loading"
          row-key="id"
          :server-side-sort="true"
          default-sort-key="name"
          default-sort-order="asc"
          @sort="handleSort"
        >
          <template #cell-id="{ value }">
            <span class="font-mono text-xs text-gray-500 dark:text-gray-400">#{{ value }}</span>
          </template>
          <template #cell-name="{ row, value }">
            <router-link
              :to="`/supplier/accounts/${row.id}`"
              class="font-medium text-blue-600 hover:underline dark:text-blue-400"
            >
              {{ value }}
            </router-link>
          </template>
          <template #cell-platform_type="{ row }">
            <PlatformTypeBadge :platform="row.platform" :type="row.type" />
          </template>
          <template #cell-status="{ row }">
            <AccountStatusIndicator :account="row" />
          </template>
          <template #cell-last_used_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ value ? formatRelativeTime(value) : '-' }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ value ? formatDateTime(value) : '-' }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <router-link
                :to="`/supplier/accounts/${row.id}`"
                class="text-sm text-blue-600 hover:underline dark:text-blue-400"
              >
                详情
              </router-link>
              <button
                v-if="row.status === 'error'"
                class="text-sm text-orange-600 hover:underline dark:text-orange-400"
                @click="clearError(row.id)"
              >
                清除错误
              </button>
              <button
                class="text-sm text-cyan-600 hover:underline dark:text-cyan-400"
                @click="refreshAccount(row.id)"
              >
                刷新
              </button>
              <button
                class="text-sm text-red-600 hover:underline dark:text-red-400"
                @click="deleteAccount(row)"
              >
                删除
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <!-- Pagination -->
      <template #pagination>
        <Pagination
          :total="total"
          :page="page"
          :page-size="pageSize"
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- 创建账号弹窗 -->
    <BaseDialog :show="showCreateModal" title="创建上游账号" width="normal" @close="showCreateModal = false">
      <form id="supplier-create-account-form" @submit.prevent="submitCreate" class="space-y-4">
        <div>
          <label class="input-label">名称</label>
          <input v-model="createForm.name" type="text" required class="input" placeholder="请输入账号名称" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">平台</label>
            <select v-model="createForm.platform" class="input">
              <option v-for="p in platforms" :key="p" :value="p">{{ p }}</option>
            </select>
          </div>
          <div>
            <label class="input-label">类型</label>
            <select v-model="createForm.type" class="input">
              <option value="apikey">API Key</option>
              <option value="oauth">OAuth</option>
              <option value="setup-token">Setup Token</option>
              <option value="upstream">Upstream</option>
            </select>
          </div>
        </div>
        <div>
          <label class="input-label">凭据 (JSON)</label>
          <textarea
            v-model="createForm.credentialsJson"
            rows="6"
            class="input font-mono text-xs"
            placeholder='{"api_key":"sk-..."}'
          />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">并发数</label>
            <input v-model.number="createForm.concurrency" type="number" min="0" class="input" />
          </div>
          <div>
            <label class="input-label">优先级</label>
            <input v-model.number="createForm.priority" type="number" min="0" class="input" />
          </div>
        </div>
        <div v-if="createError" class="rounded-md bg-red-50 p-2 text-sm text-red-600 dark:bg-red-900/30 dark:text-red-400">
          {{ createError }}
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="showCreateModal = false">取消</button>
          <button type="submit" form="supplier-create-account-form" :disabled="creating" class="btn btn-primary">
            {{ creating ? '创建中...' : '创建' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- 导入 Codex 弹窗 -->
    <BaseDialog :show="showImportModal" title="导入 Codex Session" width="normal" @close="showImportModal = false">
      <form id="supplier-import-codex-form" @submit.prevent="submitImport" class="space-y-4">
        <div>
          <label class="input-label">accessToken / Session JSON（每行一个或 JSON 数组）</label>
          <textarea
            v-model="importForm.content"
            rows="8"
            class="input font-mono text-xs"
            placeholder='eyJhbGciOi... 或 {"accessToken":"..."}'
          />
        </div>
        <div>
          <label class="input-label">账号名称前缀（可选）</label>
          <input v-model="importForm.name" type="text" class="input" />
        </div>
        <div class="flex items-center gap-2">
          <input id="supplier-update-existing" v-model="importForm.update_existing" type="checkbox" class="rounded" />
          <label for="supplier-update-existing" class="text-sm text-gray-700 dark:text-gray-300">更新已存在的账号</label>
        </div>
        <div v-if="importResult" class="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-800">
          <p>总计: {{ importResult.total }}，创建: {{ importResult.created }}，更新: {{ importResult.updated }}，跳过: {{ importResult.skipped }}，失败: {{ importResult.failed }}</p>
          <div v-if="importResult.errors?.length" class="mt-2 text-red-600 dark:text-red-400">
            <p v-for="(e, i) in importResult.errors.slice(0, 5)" :key="i">- {{ e.message }}</p>
          </div>
        </div>
        <div v-if="importError" class="rounded-md bg-red-50 p-2 text-sm text-red-600 dark:bg-red-900/30 dark:text-red-400">
          {{ importError }}
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="showImportModal = false">取消</button>
          <button type="submit" form="supplier-import-codex-form" :disabled="importing" class="btn btn-primary">
            {{ importing ? '导入中...' : '导入' }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import { supplierAPI, type SupplierAccount } from '@/api/supplier'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import type { Account } from '@/types'

const platforms = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok']

const loading = ref(false)
const accounts = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const searchInput = ref('')
const search = ref('')
const platformFilter = ref('')
const statusFilter = ref('')
const sortBy = ref('name')
const sortOrder = ref<'asc' | 'desc'>('asc')

const platformOptions = computed(() => [
  { value: '', label: '全部平台' },
  ...platforms.map(p => ({ value: p, label: p })),
])

const statusOptions = computed(() => [
  { value: '', label: '全部状态' },
  { value: 'active', label: '活跃' },
  { value: 'error', label: '错误' },
  { value: 'disabled', label: '禁用' },
])

const columns = computed(() => [
  { key: 'name', label: '名称', sortable: true },
  { key: 'id', label: 'ID', sortable: true },
  { key: 'platform_type', label: '平台/类型', sortable: false },
  { key: 'status', label: '状态', sortable: true },
  { key: 'last_used_at', label: '最后使用', sortable: true },
  { key: 'created_at', label: '创建时间', sortable: true },
  { key: 'actions', label: '操作', sortable: false },
])

// 创建账号弹窗
const showCreateModal = ref(false)
const creating = ref(false)
const createError = ref('')
const createForm = ref({
  name: '',
  platform: 'anthropic',
  type: 'apikey',
  credentialsJson: '',
  concurrency: 1,
  priority: 50,
})

// 导入 Codex 弹窗
const showImportModal = ref(false)
const importing = ref(false)
const importError = ref('')
const importResult = ref<any>(null)
const importForm = ref({
  content: '',
  name: '',
  update_existing: true,
})

const handleSearch = () => {
  search.value = searchInput.value
  page.value = 1
  loadAccounts()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortBy.value = key
  sortOrder.value = order
  loadAccounts()
}

const handlePageChange = (p: number) => {
  page.value = p
  loadAccounts()
}

const handlePageSizeChange = (size: number) => {
  pageSize.value = size
  page.value = 1
  loadAccounts()
}

const loadAccounts = async () => {
  loading.value = true
  try {
    const res = await supplierAPI.listAccounts({
      page: page.value,
      page_size: pageSize.value,
      search: search.value,
      platform: platformFilter.value,
      status: statusFilter.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
    })
    accounts.value = (res.items || []) as Account[]
    total.value = res.total || 0
  } catch (error) {
    console.error('Failed to load accounts:', error)
    accounts.value = []
  } finally {
    loading.value = false
  }
}

// 创建账号
const submitCreate = async () => {
  createError.value = ''
  if (!createForm.value.name.trim()) {
    createError.value = '请输入账号名称'
    return
  }
  let credentials: Record<string, any>
  try {
    credentials = JSON.parse(createForm.value.credentialsJson || '{}')
  } catch {
    createError.value = '凭据 JSON 格式错误'
    return
  }
  creating.value = true
  try {
    await supplierAPI.createAccount({
      name: createForm.value.name,
      platform: createForm.value.platform,
      type: createForm.value.type,
      credentials,
      concurrency: createForm.value.concurrency,
      priority: createForm.value.priority,
    })
    showCreateModal.value = false
    createForm.value = { name: '', platform: 'anthropic', type: 'apikey', credentialsJson: '', concurrency: 1, priority: 50 }
    await loadAccounts()
  } catch (error: any) {
    createError.value = error?.response?.data?.message || '创建失败'
  } finally {
    creating.value = false
  }
}

// 导入 Codex
const submitImport = async () => {
  importError.value = ''
  importResult.value = null
  if (!importForm.value.content.trim()) {
    importError.value = '请输入 accessToken 或 Session JSON'
    return
  }
  importing.value = true
  try {
    const result = await supplierAPI.importCodexSession({
      content: importForm.value.content,
      name: importForm.value.name,
      update_existing: importForm.value.update_existing,
    })
    importResult.value = result
    if (result.created > 0 || result.updated > 0) {
      await loadAccounts()
    }
  } catch (error: any) {
    importError.value = error?.response?.data?.message || '导入失败'
  } finally {
    importing.value = false
  }
}

// 账号操作
const clearError = async (id: number) => {
  try {
    await supplierAPI.clearAccountError(id)
    await loadAccounts()
  } catch (error) {
    console.error('Failed to clear error:', error)
  }
}

const refreshAccount = async (id: number) => {
  try {
    await supplierAPI.refreshAccount(id)
    await loadAccounts()
  } catch (error) {
    console.error('Failed to refresh:', error)
  }
}

const deleteAccount = async (acc: SupplierAccount) => {
  if (!confirm(`确定删除账号「${acc.name}」吗？此操作不可撤销。`)) return
  try {
    await supplierAPI.deleteAccount(acc.id)
    await loadAccounts()
  } catch (error) {
    console.error('Failed to delete:', error)
  }
}

onMounted(loadAccounts)
</script>
