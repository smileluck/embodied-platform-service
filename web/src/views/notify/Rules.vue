<template>
  <!-- 告警规则：任务失败 / 监控阈值 / IP 自动封禁 → 命中后经渠道发送（冷却去重） -->
  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['notify:rule:create']" type="primary" ghost @click="openCreate">
          {{ t('notify.rule.title') }} · {{ t('common.add') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :bordered="false" />
  </n-card>

  <n-modal v-model:show="showModal" preset="card" :title="editing ? t('common.edit') : t('common.add')" style="width: 620px" :bordered="false" size="small">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="130">
      <n-form-item :label="t('notify.rule.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="20" show-count />
      </n-form-item>
      <n-form-item :label="t('notify.rule.source')">
        <n-select v-model:value="form.source" :options="sourceOptions" />
      </n-form-item>
      <template v-if="form.source === 'monitor'">
        <n-form-item :label="t('notify.rule.metric')" path="metric">
          <n-select v-model:value="form.metric" :options="metricOptions" />
        </n-form-item>
        <n-form-item :label="t('notify.rule.threshold')" path="threshold">
          <n-input-number v-model:value="form.threshold" :min="1" :max="100" style="width: 160px">
            <template #suffix>%</template>
          </n-input-number>
        </n-form-item>
      </template>
      <n-form-item :label="t('notify.rule.channels')" path="channelIds">
        <n-select v-model:value="form.channelIds" multiple :options="channelOptions" :max-tag-count="4" :loading="channelsLoading" />
      </n-form-item>
      <n-form-item :label="t('notify.rule.cooldown')">
        <n-input-number v-model:value="form.cooldownMinutes" :min="1" :max="1440" style="width: 160px" />
        <span class="tip-inline">{{ t('notify.rule.cooldownTip') }}</span>
      </n-form-item>
      <n-form-item :label="t('notify.rule.remark')">
        <n-input v-model:value="form.remark" type="textarea" :rows="2" :maxlength="200" show-count />
      </n-form-item>
      <n-form-item :label="t('notify.rule.status')">
        <n-switch v-model:value="form.status" :checked-value="1" :unchecked-value="0" />
      </n-form-item>
    </n-form>
    <template #action>
      <div class="modal-actions">
        <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NInput, NInputNumber, NModal,
  NSelect, NSwitch, NTag, useDialog, useMessage,
  type DataTableColumns, type FormInst, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { renderActions } from '../../utils/tableActions'
import { useUserStore } from '../../stores/user'
import { createNotifyRule, deleteNotifyRule, listNotifyChannels, listNotifyRules, updateNotifyRule } from '../../api'
import type { NotifyRule } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const rows = ref<NotifyRule[]>([])
const loading = ref(false)

const sourceOptions = computed(() => [
  { label: t('notify.rule.sourceJobFailed'), value: 'job_failed' },
  { label: t('notify.rule.sourceMonitor'), value: 'monitor' },
  { label: t('notify.rule.sourceIPAutoban'), value: 'ip_autoban' },
])
const metricOptions = computed(() => [
  { label: t('notify.rule.metricCPU'), value: 'cpu' },
  { label: t('notify.rule.metricMem'), value: 'mem' },
  { label: t('notify.rule.metricSwap'), value: 'swap' },
])
const metricLabel = (m: string) => metricOptions.value.find((o) => o.value === m)?.label ?? m

// 渠道名映射（表格与回显共用；下拉仅列启用中的渠道）
const channels = ref<{ id: number; name: string; status: number }[]>([])
const channelsLoading = ref(false)
const channelOptions = computed(() =>
  channels.value.filter((c) => c.status === 1).map((c) => ({ label: c.name, value: c.id })))
const channelName = (id: number) => channels.value.find((c) => c.id === id)?.name ?? `#${id}`

async function loadChannels() {
  channelsLoading.value = true
  try {
    const res = await listNotifyChannels()
    channels.value = res.data.data.map((c) => ({ id: c.id, name: c.name, status: c.status }))
  } catch {
    // 渠道列表加载失败不打断页面，下拉为空时保存会被必选校验拦下
  } finally {
    channelsLoading.value = false
  }
}

const columns = computed<DataTableColumns<NotifyRule>>(() => [
  { title: t('notify.rule.name'), key: 'name', width: 130, ellipsis: { tooltip: true } },
  {
    title: t('notify.rule.source'), key: 'source', width: 110,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.source === 'monitor' ? 'warning' : row.source === 'ip_autoban' ? 'info' : 'error' },
      { default: () => sourceOptions.value.find((o) => o.value === row.source)?.label ?? row.source }),
  },
  {
    title: t('notify.rule.metric'), key: 'metric', width: 170,
    render: (row) => row.source === 'monitor' ? `${metricLabel(row.metric)} ≥ ${row.threshold}%` : '—',
  },
  {
    title: t('notify.rule.channels'), key: 'channels', minWidth: 150, ellipsis: { tooltip: true },
    render: (row) => row.channel_ids.map(channelName).join('、'),
  },
  { title: t('notify.rule.cooldown'), key: 'cooldown', width: 100, render: (row) => `${Math.round(row.cooldown_seconds / 60)} min` },
  {
    title: t('notify.rule.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 1 ? 'success' : 'default' },
      { default: () => row.status === 1 ? t('notify.rule.enabled') : t('notify.rule.disabled') }),
  },
  {
    title: t('notify.rule.lastTriggered'), key: 'last_triggered_at', width: 150,
    render: (row) => row.last_triggered_at ? row.last_triggered_at.slice(0, 19).replace('T', ' ') : t('notify.rule.never'),
  },
  {
    title: t('common.actions'), key: 'actions', width: 130,
    render: (row) => {
      const actions = []
      if (userStore.has('notify:rule:update')) actions.push({ label: t('common.edit'), onClick: () => openEdit(row) })
      if (userStore.has('notify:rule:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
])

async function load() {
  loading.value = true
  try {
    const res = await listNotifyRules()
    rows.value = res.data.data
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    loading.value = false
  }
}

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const saving = ref(false)
const formRef = ref<FormInst>()
const form = reactive({
  name: '',
  source: 'job_failed' as 'job_failed' | 'monitor' | 'ip_autoban',
  metric: 'cpu' as 'cpu' | 'mem' | 'swap',
  threshold: 90,
  channelIds: [] as number[],
  cooldownMinutes: 5,
  status: 1,
  remark: '',
})

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('notify.rule.nameRequired'), trigger: ['blur', 'change'] }],
  metric: form.source === 'monitor' ? [{ required: true, trigger: ['blur', 'change'] }] : [],
  threshold: form.source === 'monitor'
    ? [{ required: true, type: 'number', message: t('notify.rule.threshold'), trigger: ['blur', 'change'] }]
    : [],
  channelIds: [{ required: true, type: 'number' as any, message: t('notify.rule.channelsRequired'), trigger: ['blur', 'change'] }],
}))

function openCreate() {
  editing.value = false
  Object.assign(form, { name: '', source: 'job_failed', metric: 'cpu', threshold: 90, channelIds: [], cooldownMinutes: 5, status: 1, remark: '' })
  showModal.value = true
}

function openEdit(row: NotifyRule) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, {
    name: row.name,
    source: row.source,
    metric: (row.metric || 'cpu') as 'cpu' | 'mem' | 'swap',
    threshold: row.threshold || 90,
    channelIds: [...row.channel_ids],
    cooldownMinutes: Math.max(1, Math.round(row.cooldown_seconds / 60)),
    status: row.status,
    remark: row.remark,
  })
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  if (form.channelIds.length === 0) {
    message.warning(t('notify.rule.channelsRequired'))
    return
  }
  saving.value = true
  try {
    const data = {
      name: form.name.trim(),
      source: form.source,
      metric: (form.source === 'monitor' ? form.metric : '') as NotifyRule['metric'],
      threshold: form.source === 'monitor' ? form.threshold : 0,
      channel_ids: form.channelIds,
      cooldown_seconds: form.cooldownMinutes * 60,
      status: form.status,
      remark: form.remark,
    }
    if (editing.value) {
      await updateNotifyRule(editId.value, data)
    } else {
      await createNotifyRule(data)
    }
    message.success(t('common.success'))
    showModal.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row: NotifyRule) {
  dialog.warning({
    title: t('common.tips'),
    content: t('notify.rule.deleteConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteNotifyRule(row.id)
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

onMounted(() => {
  loadChannels()
  load()
})
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
.tip-inline {
  margin-left: 10px;
  font-size: 12px;
  opacity: 0.6;
}
</style>
