<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="tenant-roles" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('tenantRole.keywordPlaceholder')" clearable style="width: 200px" @keyup.enter="search" />
    <n-select v-model:value="query.tenant_id" :options="tenantOptions" clearable filterable :placeholder="t('tenantRole.tenantPlaceholder')" style="width: 180px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openCreate" v-permission="['tenantRole:create']">{{ t('tenantRole.newRole') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑（code 创建后不可修改；perm_codes 从平台目录勾选） -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('tenantRole.editRole') : t('tenantRole.newRole')" style="width: 560px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="90">
      <n-form-item :label="t('tenantRole.tenant')" path="tenant_id">
        <n-select v-model:value="form.tenant_id" :options="tenantOptions" filterable :disabled="editing" :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
      <n-form-item :label="t('tenantRole.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('tenantRole.code')" path="code">
        <n-input v-model:value="form.code" :maxlength="64" :disabled="editing" :placeholder="t('tenantRole.codePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('tenantRole.remark')" path="remark">
        <n-input v-model:value="form.remark" type="textarea" :rows="2" :maxlength="255" />
      </n-form-item>
      <n-form-item :label="t('tenantRole.perms')" path="perm_codes">
        <div class="perm-groups">
          <div v-for="group in permGroups" :key="group.name" class="perm-group">
            <div class="perm-group-header">
              <span>{{ t(`tenantRole.permGroup.${group.name}`) }}</span>
              <n-button size="tiny" quaternary type="primary" @click="toggleGroup(group)">{{ groupAllChecked(group) ? t('tenantRole.clearGroup') : t('tenantRole.checkGroup') }}</n-button>
            </div>
            <n-checkbox-group v-model:value="form.perm_codes">
              <n-space :size="[4, 4]">
                <n-checkbox v-for="p in group.items" :key="p" :value="p" :label="t(`tenantRole.perm.${p}`)" />
              </n-space>
            </n-checkbox-group>
          </div>
          <n-empty v-if="!permGroups.length" :description="t('tenantRole.permsEmpty')" size="small" style="margin: 8px 0" />
        </div>
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
import { NButton, NCard, NCheckbox, NCheckboxGroup, NDataTable, NEmpty, NForm, NFormItem, NInput, NModal, NSelect, NSpace, NTag, useDialog, useMessage, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { createTenantRole, deleteTenantRole, listTenantRoles, listTenantUserPerms, listTenants, updateTenantRole } from '../../api'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import type { TenantRole, TenantUserPermDef } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<TenantRole[]>([])
const query = reactive({ kw: '', tenant_id: null as number | null, page: 1, page_size: 10 })

const tenantOptions = ref<{ label: string; value: number }[]>([])

// 平台权限点目录（按 group 聚合成勾选分组）
const permCatalog = ref<TenantUserPermDef[]>([])
const permGroups = computed(() => {
  const byGroup = new Map<string, string[]>()
  for (const p of permCatalog.value) {
    if (!byGroup.has(p.group)) byGroup.set(p.group, [])
    byGroup.get(p.group)!.push(p.code)
  }
  return [...byGroup.entries()].map(([name, items]) => ({ name, items }))
})

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const form = reactive({ tenant_id: null as number | null, name: '', code: '', remark: '', perm_codes: [] as string[] })
const formRef = ref<FormInst | null>(null)

const rules = computed<FormRules>(() => ({
  tenant_id: [{ required: true, type: 'number', message: t('tenantRole.form.tenantRequired'), trigger: ['blur', 'change'] }],
  name: [{ required: true, message: t('tenantRole.form.nameRequired'), trigger: ['blur', 'input'] }],
  code: [
    { required: true, message: t('tenantRole.form.codeRequired'), trigger: ['blur', 'input'] },
    { min: 2, max: 64, message: t('tenantRole.form.codeLength'), trigger: ['blur', 'input'] },
  ],
}))

const { pagination, setTotal, runSearch: search } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listTenantRoles({
      page: query.page,
      page_size: query.page_size,
      kw: query.kw || undefined,
      tenant_id: query.tenant_id ?? undefined,
    })
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

async function loadTenantOptions() {
  try {
    const { data } = await listTenants({ page: 1, page_size: 0, status: 1 })
    tenantOptions.value = data.data.list
      .filter((x) => x.platform_id > 0)
      .map((x) => ({ label: x.name, value: x.platform_id }))
  } catch {
    // 下拉选项加载失败不阻塞页面
  }
}

async function loadPermCatalog() {
  try {
    const { data } = await listTenantUserPerms()
    permCatalog.value = data.data
  } catch {
    permCatalog.value = []
  }
}

function resetQuery() {
  query.kw = ''
  query.tenant_id = null
  query.page = 1
  load()
}

function groupAllChecked(group: { items: string[] }) {
  return group.items.every((c) => form.perm_codes.includes(c))
}

function toggleGroup(group: { items: string[] }) {
  if (groupAllChecked(group)) {
    form.perm_codes = form.perm_codes.filter((c) => !group.items.includes(c))
  } else {
    form.perm_codes = [...new Set([...form.perm_codes, ...group.items])]
  }
}

function openCreate() {
  editing.value = false
  Object.assign(form, { tenant_id: query.tenant_id, name: '', code: '', remark: '', perm_codes: [] })
  showModal.value = true
}

function openEdit(row: TenantRole) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { tenant_id: row.tenant_id, name: row.name, code: row.code, remark: row.remark, perm_codes: [...row.perm_codes] })
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateTenantRole(editId.value, {
        name: form.name.trim(),
        remark: form.remark.trim(),
        perm_codes: form.perm_codes,
      })
    } else {
      await createTenantRole({
        tenant_id: form.tenant_id!,
        name: form.name.trim(),
        code: form.code.trim(),
        remark: form.remark.trim() || undefined,
        perm_codes: form.perm_codes,
      })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantRole.saveFailed'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row: TenantRole) {
  dialog.warning({
    title: t('tenantRole.deleteConfirmTitle'),
    content: t('tenantRole.deleteConfirmContent', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteTenantRole(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('tenantRole.deleteFailed'))
      }
    },
  })
}

const tenantName = (tid: number) => tenantOptions.value.find((o) => o.value === tid)?.label || `#${tid}`

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<TenantRole>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('tenantRole.name'), key: 'name', width: 130 },
  { title: t('tenantRole.code'), key: 'code', width: 130 },
  { title: t('tenantRole.tenant'), key: 'tenant_name', width: 130, render: (row) => row.tenant_name || tenantName(row.tenant_id) },
  { title: t('tenantRole.remark'), key: 'remark', width: 140, ellipsis: { tooltip: true }, render: (row) => row.remark || '—' },
  {
    title: t('tenantRole.perms'), key: 'perm_codes', width: 220,
    render: (row) => row.perm_codes.length
      ? h('span', { style: 'display:inline-flex;gap:4px;flex-wrap:wrap' }, [
          h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => String(row.perm_codes.length) }),
          ...row.perm_codes.slice(0, 4).map((c) => h(NTag, { size: 'small', bordered: false }, { default: () => c })),
          ...(row.perm_codes.length > 4 ? [h('span', { style: 'color:var(--text-color-3)' }, '…')] : []),
        ])
      : '—',
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 140,
    render(row) {
      const actions: TableAction[] = []
      if (userStore.has('tenantRole:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('tenantRole:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(() => {
  load()
  loadTenantOptions()
  loadPermCatalog()
})
</script>

<style scoped>
/* 卡头只放操作按钮（页面标题由顶栏展示） */
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* 权限点目录分组勾选（纵向滚动防长目录撑爆弹窗） */
.perm-groups {
  width: 100%;
  max-height: 260px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
}

.perm-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
  font-size: 13px;
  font-weight: 500;
}
</style>
