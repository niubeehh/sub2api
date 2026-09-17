<template>
  <BaseDialog :show="true" :title="proxy ? '编辑代理' : '新建代理'" width="normal" @close="$emit('close')">
    <form id="supplier-proxy-form" @submit.prevent="handleSubmit" class="space-y-4">
      <div>
        <label class="input-label">名称 <span class="text-red-500">*</span></label>
        <input v-model="form.name" type="text" required class="input" placeholder="代理名称" />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">协议 <span class="text-red-500">*</span></label>
          <select v-model="form.protocol" required class="input">
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
            <option value="socks5">SOCKS5</option>
            <option value="socks5h">SOCKS5H</option>
          </select>
        </div>
        <div>
          <label class="input-label">端口 <span class="text-red-500">*</span></label>
          <input v-model.number="form.port" type="number" required min="1" max="65535" class="input" />
        </div>
      </div>
      <div>
        <label class="input-label">主机 <span class="text-red-500">*</span></label>
        <input v-model="form.host" type="text" required class="input" placeholder="IP 或域名" />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">用户名</label>
          <input v-model="form.username" type="text" class="input" />
        </div>
        <div>
          <label class="input-label">密码</label>
          <input
            v-model="form.password"
            type="text"
            class="input"
            :placeholder="proxy ? '留空则不修改密码' : ''"
            @input="passwordDirty = true"
          />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">过期时间（Unix 时间戳）</label>
          <input v-model.number="form.expires_at" type="number" class="input" placeholder="留空=永不过期" />
        </div>
        <div>
          <label class="input-label">过期预警天数</label>
          <input v-model.number="form.expiry_warn_days" type="number" min="0" class="input" />
        </div>
      </div>
      <div>
        <label class="input-label">回退模式</label>
        <select v-model="form.fallback_mode" class="input">
          <option value="none">无</option>
          <option value="proxy">切换到备用代理</option>
          <option value="direct">直连</option>
        </select>
      </div>
      <div v-if="form.fallback_mode === 'proxy'">
        <label class="input-label">备用代理 ID</label>
        <input v-model.number="form.backup_proxy_id" type="number" class="input" />
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" @click="$emit('close')" class="btn btn-secondary">取消</button>
        <button type="submit" form="supplier-proxy-form" :disabled="saving" class="btn btn-primary">
          {{ saving ? '保存中...' : '保存' }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { supplierAPI } from '@/api/supplier'

const props = defineProps<{ proxy?: any }>()
const emit = defineEmits<{ close: []; saved: [] }>()

const saving = ref(false)
// 编辑态密码原文不再回传：留空=保持不变，仅用户输入过才提交
const passwordDirty = ref(false)

const form = reactive({
  name: '',
  protocol: 'http',
  host: '',
  port: 8080,
  username: '',
  password: '',
  expires_at: null as number | null,
  fallback_mode: 'none',
  backup_proxy_id: null as number | null,
  expiry_warn_days: 7,
})

onMounted(() => {
  if (props.proxy) {
    form.name = props.proxy.name || ''
    form.protocol = props.proxy.protocol || 'http'
    form.host = props.proxy.host || ''
    form.port = props.proxy.port || 8080
    form.username = props.proxy.username || ''
    form.password = ''
    form.expires_at = props.proxy.expires_at ? Math.floor(new Date(props.proxy.expires_at).getTime() / 1000) : null
    form.fallback_mode = props.proxy.fallback_mode || 'none'
    form.backup_proxy_id = props.proxy.backup_proxy_id || null
    form.expiry_warn_days = props.proxy.expiry_warn_days || 7
  }
})

async function handleSubmit() {
  saving.value = true
  try {
    const data: {
      name: string
      protocol: string
      host: string
      port: number
      username: string
      password?: string
      expires_at: number | null
      fallback_mode: string
      backup_proxy_id: number | null
      expiry_warn_days: number
    } = {
      name: form.name,
      protocol: form.protocol,
      host: form.host,
      port: form.port,
      username: form.username,
      expires_at: form.expires_at,
      fallback_mode: form.fallback_mode,
      backup_proxy_id: form.backup_proxy_id,
      expiry_warn_days: form.expiry_warn_days,
    }
    // 新建必带密码；编辑仅当用户输入过才提交（留空=保持原密码）
    if (!props.proxy || passwordDirty.value) {
      data.password = form.password
    }
    if (props.proxy) {
      await supplierAPI.updateProxy(props.proxy.id, data)
    } else {
      await supplierAPI.createProxy(data)
    }
    emit('saved')
  } catch (e) {
    alert('保存失败: ' + (e as Error).message)
  } finally {
    saving.value = false
  }
}
</script>
