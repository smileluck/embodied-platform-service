<template>
  <!-- 定时任务：列表 + 启停/立即执行 + 执行记录抽屉 -->
  <SearchCard storage-key="jobs" @search="search" @reset="query.name = ''">
    <n-input v-model:value="query.name" :placeholder="t('job.name')" clearable style="width: 220px" @keyup.enter="search" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['job:create']" type="primary" ghost @click="openCreate">
          {{ t('job.new') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :pagination="pagination" remote :bordered="false" />
  </n-card>

  <!-- 新增/编辑 -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('job.edit') : t('job.new')" style="width: 520px">
    <n-form :model="form" :rules="rules" label-placement="left" label-width="90">
      <n-form-item :label="t('job.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="20" show-count />
      </n-form-item>
      <n-form-item :label="t('job.handler')" path="handler_key">
        <n-select v-model:value="form.handler_key" :options="handlerOptions" :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
      <n-form-item :label="t('job.cron')" path="cron">
        <n-input v-model:value="form.cron" :maxlength="32" placeholder="0 4 * * *" />
      </n-form-item>
      <n-form-item :label="t('common.remark')">
        <n-input v-model:value="form.remark" :maxlength="200" />
      </n-form-item>
      <n-form-item :label="t('common.status')">
        <n-switch v-model:value="form.status" :checked-value="1" :unchecked-value="0" size="small" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 执行记录 -->
  <n-drawer v-model:show="showLogs" :width="520" :auto-focus="false">
    <n-drawer-content :title="`${t('job.logs')} · ${logJob?.name ?? ''}`" closable>
      <n-data-table size="small" :columns="logColumns" :data="logRows" :loading="logsLoading" :bordered="false" />
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NDrawer, NDrawerContent, NForm, NFormItem, NInput, NModal,
  NSelect, NSwitch, NTag, useDialog, useMessage,
  type DataTableColumns, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { createJob, deleteJob, listJobHandlers, listJobLogs, listJobs, runJobOnce, setJobStatus, updateJob } from '../../api'
import type { JobHandler, JobInfo, JobLog } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const query = reactive({ name: '', page: 1, page_size: 10 })
const rows = ref<JobInfo[]>([])
const loading = ref(false)
const { pagination, setTotal, runSearch: search } = usePagination(query, load)

const handlerOptions = ref<{ label: string; value: string }[]>([])

const columns = computed<DataTableColumns<JobInfo>>(() => [
  { title: t('job.name'), key: 'name', width: 150 },
  { title: t('job.cron'), key: 'cron', width: 120, className: 'mono' },
  { title: t('job.handler'), key: 'handler_key', width: 180, render: (row) => handlerLabel(row.handler_key) },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 1 ? 'success' : 'default' },
      { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  {
    title: t('job.lastRun'), key: 'last_run_at', width: 150,
    render: (row) => (row.last_run_at ? row.last_run_at.slice(0, 19).replace('T', ' ') : '—'),
  },
  { title: t('common.remark'), key: 'remark', ellipsis: { tooltip: true } },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => {
      const actions = []
      if (userStore.has('job:run')) actions.push({ label: t('job.runNow'), onClick: () => runNow(row) })
      if (userStore.has('job:log:list')) actions.push({ label: t('job.logs'), onClick: () => openLogs(row) })
      if (userStore.has('job:status')) actions.push({ label: row.status === 1 ? t('common.disable') : t('common.enable'), onClick: () => toggle(row) })
      if (userStore.has('job:update')) actions.push({ label: t('common.edit'), onClick: () => openEdit(row) })
      if (userStore.has('job:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
])

function handlerLabel(key: string): string {
  const found = handlerOptions.value.find((h) => h.value === key)
  return found?.label ?? key
}

async function load() {
  loading.value = true
  try {
    const res = await listJobs({ page: query.page, page_size: query.page_size, name: query.name || undefined })
    rows.value = res.data.data.list
    setTotal(res.data.data.page.total)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  load()
  if (userStore.has('job:handler:list')) {
    try {
      const res = await listJobHandlers()
      handlerOptions.value = (res.data.data ?? []).map((h: JobHandler) => ({
        label: `${h.key}（${h.description}）`, value: h.key,
      }))
    } catch { /* 处理器清单加载失败不阻断页面 */ }
  }
})

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const saving = ref(false)
const form = reactive({ name: '', cron: '', handler_key: '', remark: '', status: 1 })

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('job.nameRequired'), trigger: ['blur', 'change'] }],
  cron: [{ required: true, message: t('job.cronRequired'), trigger: ['blur', 'change'] }],
  handler_key: [{ required: true, message: t('job.handlerRequired'), trigger: ['blur', 'change'] }],
}))

function openCreate() {
  editing.value = false
  Object.assign(form, { name: '', cron: '', handler_key: '', remark: '', status: 1 })
  showModal.value = true
}

function openEdit(row: JobInfo) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { name: row.name, cron: row.cron, handler_key: row.handler_key, remark: row.remark, status: row.status })
  showModal.value = true
}

async function save() {
  saving.value = true
  try {
    if (editing.value) {
      await updateJob(editId.value, { ...form })
    } else {
      await createJob({ ...form })
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

async function toggle(row: JobInfo) {
  try {
    await setJobStatus(row.id, row.status === 1 ? 0 : 1)
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  }
}

async function runNow(row: JobInfo) {
  try {
    await runJobOnce(row.id)
    message.success(t('job.runSubmitted'))
    setTimeout(load, 1200)
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  }
}

function confirmDelete(row: JobInfo) {
  dialog.warning({
    title: t('common.tips'),
    content: t('job.deleteConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteJob(row.id)
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

// ---- 执行记录 ----
const showLogs = ref(false)
const logJob = ref<JobInfo | null>(null)
const logRows = ref<JobLog[]>([])
const logsLoading = ref(false)

const logColumns = computed<DataTableColumns<JobLog>>(() => [
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 'success' ? 'success' : 'error' },
      { default: () => (row.status === 'success' ? t('job.runSuccess') : t('job.runFailed')) }),
  },
  { title: t('job.duration'), key: 'duration_ms', width: 90, render: (row) => `${row.duration_ms}ms` },
  { title: t('job.startedAt'), key: 'started_at', width: 150, render: (row) => row.started_at.slice(0, 19).replace('T', ' ') },
  { title: t('job.output'), key: 'output', ellipsis: { tooltip: true } },
])

async function openLogs(row: JobInfo) {
  logJob.value = row
  showLogs.value = true
  logsLoading.value = true
  try {
    const res = await listJobLogs(row.id, { page: 1, page_size: 50 })
    logRows.value = res.data.data.list
  } finally {
    logsLoading.value = false
  }
}
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}
.mono {
  font-family: var(--sx-font-mono);
}
</style>
