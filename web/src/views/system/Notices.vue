<template>
  <!-- 消息通知管理：搜索独立卡片 + 列表卡片头右侧「发布消息」（对齐用户管理页风格） -->
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

  <!-- 发布/编辑消息：卡片式弹窗（背景随亮暗主题走 cardColor，与整站面板一致） -->
  <n-modal v-model:show="showModal" preset="card" :title="editing ? t('notice.edit') : t('notice.new')" style="width: 680px" :bordered="false" size="small">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="150">
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
      <n-form-item :label="t('notice.scope')">
        <n-radio-group v-model:value="form.scope" size="small">
          <n-radio-button value="all">{{ t('notice.scopeAll') }}</n-radio-button>
          <n-radio-button value="roles">{{ t('notice.scopeRoles') }}</n-radio-button>
          <n-radio-button value="users">{{ t('notice.scopeUsers') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-form-item v-if="form.scope === 'roles'" :label="t('notice.roleTargets')">
        <n-select v-model:value="form.roleIds" multiple :options="roleOptions" :max-tag-count="6" :placeholder="t('notice.roleTargets')" style="width: 100%" />
      </n-form-item>
      <n-form-item v-if="form.scope === 'users'" :label="t('notice.userTargets')">
        <n-select
          v-model:value="form.userIds" multiple filterable remote :options="userOptions"
          :loading="userSearching" :max-tag-count="6" :placeholder="t('notice.searchUserPlaceholder')"
          style="width: 100%" @search="onSearchUsers"
        />
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
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NDatePicker, NForm, NFormItem, NInput, NModal,
  NRadioButton, NRadioGroup, NSelect, NTag, NTooltip, useDialog, useMessage,
  type DataTableColumns, type FormInst, type FormRules, type SelectOption,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { createNotice, deleteNotice, listNoticeRoleOptions, listNoticeUserOptions, listNotices, updateNotice } from '../../api'
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
  { title: t('notice.titleField'), key: 'title', minWidth: 160, ellipsis: { tooltip: true } },
  {
    title: t('notice.level'), key: 'level', width: 80,
    render: (row) => h(NTag, {
      size: 'small', bordered: false,
      type: row.level === 'important' ? 'error' : row.level === 'warning' ? 'warning' : 'info',
    }, { default: () => levelOptions.value.find((l) => l.value === row.level)?.label ?? row.level }),
  },
  {
    title: t('notice.scope'), key: 'scope', width: 110,
    render: (row) => {
      if (row.scope === 'roles' || row.scope === 'users') {
        const byRole = row.scope === 'roles'
        const names = byRole ? (row.role_names ?? []) : (row.user_names ?? [])
        const count = (byRole ? row.role_ids : row.user_ids)?.length ?? names.length
        const tag = h(NTag, { size: 'small', bordered: false, type: byRole ? 'info' : 'success' }, {
          default: () => `${byRole ? t('notice.scopeRoles') : t('notice.scopeUsers')} · ${count}`,
        })
        const tip = byRole ? names.join('、') : t('notice.usersCount', { n: count })
        return h(NTooltip, null, { trigger: () => tag, default: () => tip })
      }
      return h(NTag, { size: 'small', bordered: false }, { default: () => t('notice.scopeAll') })
    },
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
    title: t('common.actions'), key: 'actions', width: 160,
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
const formRef = ref<FormInst>()
const form = reactive<{
  title: string; content: string; level: 'info' | 'warning' | 'important'
  scope: 'all' | 'roles' | 'users'; roleIds: number[]; userIds: number[]
  publishAt: number | null; expireAt: number | null
}>({
  title: '', content: '', level: 'info', scope: 'all', roleIds: [], userIds: [], publishAt: null, expireAt: null,
})

const rules = computed<FormRules>(() => ({
  title: [{ required: true, message: t('notice.titleRequired'), trigger: ['blur', 'change'] }],
  content: [{ required: true, message: t('notice.contentRequired'), trigger: ['blur', 'change'] }],
}))

// 角色选项：懒加载一次（含超管角色在内的全部角色）
const roleOptions = ref<SelectOption[]>([])
async function ensureRoleOptions() {
  if (roleOptions.value.length) return
  try {
    const res = await listNoticeRoleOptions()
    roleOptions.value = res.data.data.map((r) => ({ label: r.name, value: r.id }))
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  }
}

// 用户选项：远程搜索（300ms 防抖，只列启用用户）
const userOptions = ref<SelectOption[]>([])
const userSearching = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | undefined
function onSearchUsers(kw: string) {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    userSearching.value = true
    try {
      const res = await listNoticeUserOptions(kw.trim())
      userOptions.value = res.data.data.map((u) => ({
        label: u.nickname ? `${u.nickname}（${u.username}）` : u.username, value: u.id,
      }))
    } catch (e: any) {
      message.error(e?.response?.data?.msg || t('common.failed'))
    } finally {
      userSearching.value = false
    }
  }, 300)
}
onBeforeUnmount(() => { if (searchTimer) clearTimeout(searchTimer) })

// 编辑回显：已选用户若无选项（未被当前搜索命中），用回显名称补齐选项
function seedUserOptions(row: NoticeInfo) {
  const known = new Set(userOptions.value.map((o) => o.value))
  ;(row.user_ids ?? []).forEach((id, i) => {
    const name = row.user_names?.[i]
    if (!known.has(id) && name) userOptions.value.push({ label: name, value: id })
  })
}

async function openCreate() {
  editing.value = false
  Object.assign(form, { title: '', content: '', level: 'info', scope: 'all', roleIds: [], userIds: [], publishAt: null, expireAt: null })
  await ensureRoleOptions()
  showModal.value = true
}

async function openEdit(row: NoticeInfo) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, {
    title: row.title, content: row.content, level: row.level,
    scope: row.scope ?? 'all',
    roleIds: [...(row.role_ids ?? [])],
    userIds: [...(row.user_ids ?? [])],
    publishAt: row.publish_at ? new Date(row.publish_at).getTime() : null,
    expireAt: row.expire_at ? new Date(row.expire_at).getTime() : null,
  })
  seedUserOptions(row)
  await ensureRoleOptions()
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  // 定向范围至少一个目标（后端同款兜底校验）
  if (form.scope === 'roles' && form.roleIds.length === 0) {
    message.warning(t('notice.roleRequired'))
    return
  }
  if (form.scope === 'users' && form.userIds.length === 0) {
    message.warning(t('notice.userRequired'))
    return
  }
  saving.value = true
  try {
    const data = {
      title: form.title.trim(),
      content: form.content,
      level: form.level,
      scope: form.scope,
      role_ids: form.scope === 'roles' ? form.roleIds : [],
      user_ids: form.scope === 'users' ? form.userIds : [],
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
