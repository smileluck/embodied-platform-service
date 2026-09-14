<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="tenants" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.name" :placeholder="t('tenant.name')" clearable style="width: 180px" @keyup.enter="load" />
    <n-input v-model:value="query.code" :placeholder="t('tenant.code')" clearable style="width: 160px" @keyup.enter="load" />
    <n-select v-model:value="query.status" :options="statusOptions" clearable :placeholder="t('tenant.statusPlaceholder')" style="width: 120px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openCreate" v-permission="['tenant:create']">{{ t('tenant.newTenant') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑（code 创建后不可修改） -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('tenant.editTenant') : t('tenant.newTenant')" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('tenant.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="64" :placeholder="t('tenant.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('tenant.code')" path="code">
        <n-input v-model:value="form.code" :maxlength="64" :disabled="editing" :placeholder="t('tenant.codePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('tenant.contactName')" path="contact_name">
        <n-input v-model:value="form.contact_name" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('tenant.contactPhone')" path="contact_phone">
        <n-input v-model:value="form.contact_phone" :maxlength="32" />
      </n-form-item>
      <n-form-item :label="t('common.remark')" path="remark">
        <n-input v-model:value="form.remark" type="textarea" :maxlength="255" :autosize="{ minRows: 2, maxRows: 4 }" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NCard, NInput, NButton, NDataTable, NModal, NForm, NFormItem, NSelect, NTag, useMessage, useDialog, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { createTenant, deleteTenant, listTenants, setTenantStatus, updateTenant } from '../../api'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import type { Tenant } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<Tenant[]>([])
const query = reactive({ name: '', code: '', status: null as number | null, page: 1, page_size: 10 })

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const form = reactive({ name: '', code: '', contact_name: '', contact_phone: '', remark: '' })
const formRef = ref<FormInst | null>(null)

// 租户状态：1 启用 0 禁用（与商户的 1/2 不同）
const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 0 },
])

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('tenant.form.nameRequired'), trigger: ['blur', 'input'] }],
  code: [
    { required: true, message: t('tenant.form.codeRequired'), trigger: ['blur', 'input'] },
    { min: 2, max: 64, message: t('tenant.form.codeLength'), trigger: ['blur', 'input'] },
  ],
}))

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listTenants({
      page: query.page,
      page_size: query.page_size,
      name: query.name || undefined,
      code: query.code || undefined,
      status: query.status ?? undefined,
    })
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.name = ''
  query.code = ''
  query.status = null
  query.page = 1
  load()
}

function openCreate() {
  editing.value = false
  Object.assign(form, { name: '', code: '', contact_name: '', contact_phone: '', remark: '' })
  showModal.value = true
}

function openEdit(row: Tenant) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { name: row.name, code: row.code, contact_name: row.contact_name, contact_phone: row.contact_phone, remark: row.remark })
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return // 校验失败，错误已在表单项上展示
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateTenant(editId.value, {
        name: form.name.trim(),
        contact_name: form.contact_name.trim(),
        contact_phone: form.contact_phone.trim(),
        remark: form.remark.trim(),
      })
    } else {
      await createTenant({
        name: form.name.trim(),
        code: form.code.trim(),
        contact_name: form.contact_name.trim() || undefined,
        contact_phone: form.contact_phone.trim() || undefined,
        remark: form.remark.trim() || undefined,
      })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenant.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function toggleStatus(row: Tenant) {
  try {
    await setTenantStatus(row.id, row.status === 1 ? 0 : 1)
    message.success(t('tenant.statusUpdated'))
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenant.statusFailed'))
  }
}

function confirmDelete(row: Tenant) {
  dialog.warning({
    title: t('tenant.deleteConfirmTitle'),
    content: t('tenant.deleteConfirmContent', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteTenant(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('tenant.deleteFailed'))
      }
    },
  })
}

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<Tenant>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('tenant.name'), key: 'name' },
  { title: t('tenant.code'), key: 'code', width: 140 },
  { title: t('tenant.contactName'), key: 'contact_name', width: 120, render: (row) => row.contact_name || '—' },
  { title: t('tenant.contactPhone'), key: 'contact_phone', width: 140, render: (row) => row.contact_phone || '—' },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 200,
    render(row) {
      const actions: TableAction[] = []
      if (userStore.has('tenant:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('tenant:update')) {
        actions.push({ label: row.status === 1 ? t('common.disable') : t('common.enable'), onClick: () => toggleStatus(row) })
      }
      if (userStore.has('tenant:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(load)
</script>

<style scoped>
/* 卡头只放操作按钮（页面标题由顶栏展示） */
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
