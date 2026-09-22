<template>
  <!-- 通知公告管理：搜索独立卡片 + 列表卡片头右侧「发布公告」（对齐用户管理页风格） -->
  <SearchCard storage-key="notices" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.title" :placeholder="t('notice.titleField')" clearable style="width: 200px" @keyup.enter="search" />
    <n-select v-model:value="query.level" :options="levelOptions" :placeholder="t('notice.level')" clearable style="width: 130px" />
    <n-select v-model:value="query.status" :options="statusOptions" :placeholder="t('notice.status')" clearable style="width: 130px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['notice:create']" type="primary" ghost @click="openCreate">
          {{ t('notice.new') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :pagination="pagination" remote :bordered="false" />
  </n-card>

  <!-- 发布/编辑公告：卡片式弹窗（背景随亮暗主题走 cardColor，与整站面板一致） -->
  <n-modal v-model:show="showModal" preset="card" :title="editing ? t('notice.edit') : t('notice.new')" style="width: 680px" :bordered="false" size="small">
    <n-form :model="form" :rules="rules" label-placement="left" label-width="150">
      <n-form-item :label="t('notice.titleField')" path="title">
        <n-input v-model:value="form.title" :maxlength="100" show-count />
      </n-form-item>
      <n-form-item :label="t('notice.level')">
        <n-radio-group v-model:value="form.level" size="small">
          <n-radio-button value="info">{{ t('notice.levelInfo') }}</n-radio-button>
          <n-radio-button value="warning">{{ t('notice.levelWarning') }}</n-radio-button>
          <n-radio-button value="important">{{ t('notice.levelImportant') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-form-item :label="t('notice.content')" path="content">
        <n-input v-model:value="form.content" type="textarea" :rows="8" :maxlength="8000" show-count />
      </n-form-item>
      <n-form-item :label="t('notice.publishAt')">
        <n-date-picker v-model:value="form.publishAt" type="datetime" clearable style="width: 100%" />
      </n-form-item>
      <n-form-item :label="t('notice.expireAt')">
        <n-date-picker v-model:value="form.expireAt" type="datetime" clearable style="width: 100%" />
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
  NButton, NCard, NDataTable, NDatePicker, NForm, NFormItem, NInput, NModal,
  NRadioButton, NRadioGroup, NSelect, NTag, useDialog, useMessage,
  type DataTableColumns, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { createNotice, deleteNotice, listNotices, updateNotice } from '../../api'
import type { NoticeInfo } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const query = reactive({ title: '', level: '', status: '', page: 1, page_size: 10 })
const rows = ref<NoticeInfo[]>([])
const loading = ref(false)
const { pagination, setTotal, runSearch: search } = usePagination(query, () => load())

const levelOptions = computed(() => [
  { label: t('notice.levelInfo'), value: 'info' },
  { label: t('notice.levelWarning'), value: 'warning' },
  { label: t('notice.levelImportant'), value: 'important' },
])
const statusOptions = computed(() => [
  { label: t('notice.statusActive'), value: 'active' },
  { label: t('notice.statusPending'), value: 'pending' },
  { label: t('notice.statusExpired'), value: 'expired' },
])

const columns = computed<DataTableColumns<NoticeInfo>>(() => [
  { title: t('notice.titleField'), key: 'title', minWidth: 180, ellipsis: { tooltip: true } },
  {
    title: t('notice.level'), key: 'level', width: 90,
    render: (row) => h(NTag, {
      size: 'small', bordered: false,
      type: row.level === 'important' ? 'error' : row.level === 'warning' ? 'warning' : 'info',
    }, { default: () => levelOptions.value.find((l) => l.value === row.level)?.label ?? row.level }),
  },
  {
    title: t('notice.publishAt'), key: 'publish_at', width: 150,
    render: (row) => row.publish_at?.slice(0, 16).replace('T', ' ') ?? '—',
  },
  {
    title: t('notice.expireAt'), key: 'expire_at', width: 150,
    render: (row) => (row.expire_at ? row.expire_at.slice(0, 16).replace('T', ' ') : t('notice.longTerm')),
  },
  { title: t('notice.creator'), key: 'creator_name', width: 100 },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => {
      const actions = []
      if (userStore.has('notice:update')) actions.push({ label: t('common.edit'), onClick: () => openEdit(row) })
      if (userStore.has('notice:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
])

async function load() {
  loading.value = true
  try {
    const res = await listNotices({
      page: query.page, page_size: query.page_size,
      title: query.title || undefined, level: query.level || undefined, status: query.status || undefined,
    })
    rows.value = res.data.data.list
    setTotal(res.data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, { title: '', level: '', status: '', page: 1 })
  load()
}

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const saving = ref(false)
const form = reactive<{ title: string; content: string; level: 'info' | 'warning' | 'important'; publishAt: number | null; expireAt: number | null }>({
  title: '', content: '', level: 'info', publishAt: null, expireAt: null,
})

const rules = computed<FormRules>(() => ({
  title: [{ required: true, message: t('notice.titleRequired'), trigger: ['blur', 'change'] }],
  content: [{ required: true, message: t('notice.contentRequired'), trigger: ['blur', 'change'] }],
}))

function openCreate() {
  editing.value = false
  Object.assign(form, { title: '', content: '', level: 'info', publishAt: null, expireAt: null })
  showModal.value = true
}

function openEdit(row: NoticeInfo) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, {
    title: row.title, content: row.content, level: row.level,
    publishAt: row.publish_at ? new Date(row.publish_at).getTime() : null,
    expireAt: row.expire_at ? new Date(row.expire_at).getTime() : null,
  })
  showModal.value = true
}

async function save() {
  saving.value = true
  try {
    const data = {
      title: form.title.trim(),
      content: form.content,
      level: form.level,
      publish_at: form.publishAt ? new Date(form.publishAt).toISOString() : '',
      expire_at: form.expireAt ? new Date(form.expireAt).toISOString() : '',
    }
    if (editing.value) {
      await updateNotice(editId.value, data)
    } else {
      await createNotice(data)
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

function confirmDelete(row: NoticeInfo) {
  dialog.warning({
    title: t('common.tips'),
    content: t('notice.deleteConfirm', { title: row.title }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteNotice(row.id)
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

onMounted(load)
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
</style>
