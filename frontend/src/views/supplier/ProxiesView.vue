<template>
  <AppLayout>
    <TablePageLayout>
      <!-- Actions -->
      <template #actions>
        <div class="flex items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <button class="btn btn-primary" @click="showCreateModal = true">
              <Icon name="plus" size="sm" class="mr-1.5" />
              新建代理
            </button>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="loadProxies">
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
            placeholder="搜索代理名称..."
            class="input w-full sm:w-64"
            @keyup.enter="handleSearch"
          />
          <Select
            v-model="protocolFilter"
            :options="protocolOptions"
            class="w-full sm:w-40"
            @change="loadProxies"
          />
          <Select
            v-model="statusFilter"
            :options="statusOptions"
            class="w-full sm:w-32"
            @change="loadProxies"
          />
          <button class="btn btn-secondary" @click="handleSearch">查询</button>
        </div>
      </template>

      <!-- Table -->
      <template #table>
        <DataTable
          :columns="columns"
          :data="proxies"
          :loading="loading"
          row-key="id"
          :server-side-sort="true"
          default-sort-key="id"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-id="{ value }">
            <span class="font-mono text-xs text-gray-500 dark:text-gray-400">#{{ value }}</span>
          </template>
          <template #cell-name="{ value }">
            <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
          </template>
          <template #cell-address="{ row }">
            <span class="font-mono text-sm text-gray-600 dark:text-gray-400">{{ row.host }}:{{ row.port }}</span>
          </template>
          <template #cell-account_count="{ value }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ value || 0 }}</span>
          </template>
          <template #cell-expires_at="{ value }">
            <span v-if="value" class="text-sm text-gray-500 dark:text-gray-400">{{ formatDate(value) }}</span>
            <span v-else class="text-sm text-gray-400">永不过期</span>
          </template>
          <template #cell-status="{ value }">
            <span :class="statusBadgeClass(value)">{{ value }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button class="text-sm text-blue-600 hover:underline dark:text-blue-400" :disabled="testing" @click="handleTest(row.id)">
                测试
              </button>
              <button class="text-sm text-blue-600 hover:underline dark:text-blue-400" @click="handleEdit(row)">
                编辑
              </button>
              <button class="text-sm text-red-600 hover:underline dark:text-red-400" @click="handleDelete(row)">
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

    <!-- 创建/编辑代理弹窗 -->
    <ProxyFormModal
      v-if="showCreateModal || showEditModal"
      :proxy="editingProxy"
      @close="closeModal"
      @saved="handleSaved"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ProxyFormModal from '@/components/supplier/ProxyFormModal.vue'
import { supplierAPI } from '@/api/supplier'

const loading = ref(false)
const testing = ref(false)
const proxies = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const searchInput = ref('')
const search = ref('')
const protocolFilter = ref('')
const statusFilter = ref('')
const sortBy = ref('id')
const sortOrder = ref<'asc' | 'desc'>('desc')
const showCreateModal = ref(false)
const showEditModal = ref(false)
const editingProxy = ref<any>(null)

const protocolOptions = computed(() => [
  { value: '', label: '全部协议' },
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
  { value: 'socks5', label: 'SOCKS5' },
  { value: 'socks5h', label: 'SOCKS5H' },
])

const statusOptions = computed(() => [
  { value: '', label: '全部状态' },
  { value: 'active', label: '活跃' },
  { value: 'inactive', label: '停用' },
])

const columns = computed(() => [
  { key: 'id', label: 'ID', sortable: true },
  { key: 'name', label: '名称', sortable: true },
  { key: 'protocol', label: '协议', sortable: true },
  { key: 'address', label: '地址', sortable: false },
  { key: 'account_count', label: '关联账号', sortable: true },
  { key: 'expires_at', label: '过期时间', sortable: true },
  { key: 'status', label: '状态', sortable: true },
  { key: 'actions', label: '操作', sortable: false },
])

function statusBadgeClass(status: string) {
  return status === 'active'
    ? 'inline-flex items-center rounded-md bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700 ring-1 ring-green-200 dark:bg-green-900/30 dark:text-green-400 dark:ring-green-800'
    : 'inline-flex items-center rounded-md bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:ring-gray-700'
}

function formatDate(date: string) {
  return new Date(date).toLocaleDateString('zh-CN')
}

const handleSearch = () => {
  search.value = searchInput.value
  page.value = 1
  loadProxies()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortBy.value = key
  sortOrder.value = order
  loadProxies()
}

const handlePageChange = (p: number) => {
  page.value = p
  loadProxies()
}

const handlePageSizeChange = (size: number) => {
  pageSize.value = size
  page.value = 1
  loadProxies()
}

async function loadProxies() {
  loading.value = true
  try {
    const res = await supplierAPI.listProxies({
      protocol: protocolFilter.value,
      status: statusFilter.value,
      search: search.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      page: page.value,
      page_size: pageSize.value,
    })
    proxies.value = res.items || []
    total.value = res.total || 0
  } catch (e) {
    console.error('加载代理失败', e)
    proxies.value = []
  } finally {
    loading.value = false
  }
}

async function handleTest(id: number) {
  testing.value = true
  try {
    await supplierAPI.testProxy(id)
    alert('测试成功')
  } catch (e) {
    alert('测试失败: ' + (e as Error).message)
  } finally {
    testing.value = false
  }
}

function handleEdit(proxy: any) {
  editingProxy.value = proxy
  showEditModal.value = true
}

async function handleDelete(proxy: any) {
  if (!confirm(`确定删除代理「${proxy.name}」吗？`)) return
  try {
    await supplierAPI.deleteProxy(proxy.id)
    await loadProxies()
  } catch (e) {
    alert('删除失败: ' + (e as Error).message)
  }
}

function closeModal() {
  showCreateModal.value = false
  showEditModal.value = false
  editingProxy.value = null
}

async function handleSaved() {
  closeModal()
  await loadProxies()
}

onMounted(() => {
  loadProxies()
})
</script>
