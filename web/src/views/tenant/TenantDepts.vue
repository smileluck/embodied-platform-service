<template>
  <!-- 搜索栏独立卡片：部门树按租户切换（部门总是挂在某个租户下） -->
  <SearchCard storage-key="tenant-depts" @search="search" @reset="resetQuery">
    <n-select v-model:value="query.tenant_id" :options="tenantOptions" clearable filterable :placeholder="t('tenantDept.tenantPlaceholder')" style="width: 220px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button class="expand-toggle" @click="toggleExpand">{{ expandAll ? t('tenantDept.collapseAll') : t('tenantDept.expandAll') }}</n-button>
        <n-button type="primary" ghost :disabled="!query.tenant_id" @click="openCreate(null)" v-permission="['tenantDept:create']">{{ t('tenantDept.newDept') }}</n-button>
      </div>
    </template>

    <!-- 未选租户时提示先选租户；树表受控展开（default-expand-all 对异步数据不生效） -->
    <n-empty v-if="!query.tenant_id" :description="t('tenantDept.selectTenantFirst')" style="padding: 48px 0" />
    <n-data-table v-else :columns="columns" :data="tree" :loading="loading" :row-key="rowKey"
      :expanded-row-keys="expandedKeys" @update:expanded-row-keys="onExpandUpdate" />
  </n-card>

  <!-- 新增/编辑（code/所属租户创建后不可修改；父级不可选自身及后代——服务端防环兜底） -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('tenantDept.editDept') : t('tenantDept.newDept')" style="width: 520px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="90">
      <n-form-item :label="t('tenantDept.tenant')" path="tenant_id">
        <n-select v-model:value="form.tenant_id" :options="tenantOptions" filterable :disabled="editing" :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
      <n-form-item :label="t('tenantDept.parent')" path="parent_id">
        <n-tree-select
          v-model:value="form.parent_id" :options="parentOptions"
          clearable :placeholder="t('tenantDept.parentPlaceholderRoot')" key-field="key" label-field="label" children-field="children"
        />
      </n-form-item>
      <n-form-item :label="t('tenantDept.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('tenantDept.code')" path="code">
        <n-input v-model:value="form.code" :maxlength="64" :disabled="editing" :placeholder="t('tenantDept.codePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('tenantDept.sort')" path="sort">
        <n-input-number v-model:value="form.sort" :min="0" style="width: 100%" />
      </n-form-item>
      <n-form-item :label="t('tenantDept.remark')" path="remark">
        <n-input v-model:value="form.remark" type="textarea" :rows="2" :maxlength="255" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 部门成员：按部门（含子部门）反查租户用户，分页展示 -->
  <n-modal v-model:show="showMembers" preset="card" :title="t('tenantDept.membersTitle')" style="width: 720px">
    <n-data-table :columns="memberColumns" :data="memberRows" :loading="memberLoading" :pagination="memberPagination" paginate-single-page remote />
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import { NButton, NCard, NDataTable, NEmpty, NForm, NFormItem, NInput, NInputNumber, NModal, NSelect, NTag, NTreeSelect, useDialog, useMessage, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { createTenantDept, deleteTenantDept, listTenantDepts, listTenantUsers, listTenants, updateTenantDept } from '../../api'
import { useUserStore } from '../../stores/user'
import type { TenantDept, TenantUserAccount } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const query = reactive({ tenant_id: null as number | null })

// 租户下拉选项（启用状态已同步租户，平台租户 ID 口径）
const tenantOptions = ref<{ label: string; value: number }[]>([])
const all = ref<TenantDept[]>([])

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const form = reactive({ tenant_id: null as number | null, parent_id: null as number | null, name: '', code: '', sort: 0, remark: '' })
const formRef = ref<FormInst | null>(null)

const rules = computed<FormRules>(() => ({
  tenant_id: [{ required: true, type: 'number', message: t('tenantDept.form.tenantRequired'), trigger: ['blur', 'change'] }],
  name: [{ required: true, message: t('tenantDept.form.nameRequired'), trigger: ['blur', 'input'] }],
  code: [
    { required: true, message: t('tenantDept.form.codeRequired'), trigger: ['blur', 'input'] },
    { min: 2, max: 64, message: t('tenantDept.form.codeLength'), trigger: ['blur', 'input'] },
  ],
}))

async function search() {
  await load()
}

async function load() {
  if (!query.tenant_id) {
    all.value = []
    return
  }
  loading.value = true
  try {
    const { data } = await listTenantDepts(query.tenant_id)
    all.value = data.data
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantDept.saveFailed'))
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

function resetQuery() {
  query.tenant_id = null
  all.value = []
}

// 平铺 -> 树（后端返回平表，前端按 parent_id 组树；sort 后 id 稳定排序）
type DeptNode = TenantDept & { children?: DeptNode[] }
const tree = ref<DeptNode[]>([])
function buildTree() {
  const build = (parentID: number): DeptNode[] =>
    all.value
      .filter((d) => d.parent_id === parentID)
      .sort((a, b) => a.sort - b.sort || a.id - b.id)
      .map((d) => {
        const children = build(d.id)
        const n: DeptNode = { ...d }
        if (children.length) n.children = children
        return n
      })
  tree.value = build(0)
  if (expandAll.value) expandedKeys.value = collectParentIds(tree.value)
}

async function refresh() {
  await load()
  buildTree()
}

// 树表受控展开：default-expand-all 对异步加载后才渲染的行不生效，手动维护展开行键
const rowKey = (row: DeptNode) => row.id
const expandedKeys = ref<number[]>([])
const expandAll = ref(true)
function collectParentIds(nodes: DeptNode[]): number[] {
  return nodes.flatMap((n) => (n.children?.length ? [n.id, ...collectParentIds(n.children)] : []))
}
function onExpandUpdate(keys: Array<string | number>) {
  expandedKeys.value = keys as number[]
}
function toggleExpand() {
  expandAll.value = !expandAll.value
  expandedKeys.value = expandAll.value ? collectParentIds(tree.value) : []
}

// 收集节点全部后代 id（父级选项禁用自身及后代，防环由前端先行、服务端兜底）
function descendantIds(id: number): Set<number> {
  const out = new Set<number>()
  let growing = true
  while (growing) {
    growing = false
    for (const n of all.value) {
      if (!out.has(n.id) && (n.parent_id === id || (n.parent_id !== 0 && out.has(n.parent_id)))) {
        out.add(n.id)
        growing = true
      }
    }
  }
  return out
}

// 父级选项：当前表单租户下的部门树（编辑时禁用自身及后代）
const parentOptions = ref<any[]>([])
function buildParentOptions() {
  const tid = form.tenant_id
  if (!tid) {
    parentOptions.value = []
    return
  }
  const banned = editing.value ? descendantIds(editId.value) : new Set<number>()
  const nodes = all.value.filter((d) => d.tenant_id === tid)
  const build = (parentID: number): any[] =>
    nodes
      .filter((d) => d.parent_id === parentID)
      .sort((a, b) => a.sort - b.sort || a.id - b.id)
      .map((d) => {
        const children = build(d.id)
        const n: any = { label: d.name, key: d.id, disabled: banned.has(d.id) || (editing.value && d.id === editId.value) }
        if (children.length) n.children = children
        return n
      })
  parentOptions.value = build(0)
}

function openCreate(parent: DeptNode | null) {
  editing.value = false
  Object.assign(form, { tenant_id: query.tenant_id, parent_id: parent?.id ?? null, name: '', code: '', sort: 0, remark: '' })
  buildParentOptions()
  showModal.value = true
}

function openEdit(row: DeptNode) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { tenant_id: row.tenant_id, parent_id: row.parent_id || null, name: row.name, code: row.code, sort: row.sort, remark: row.remark })
  buildParentOptions()
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
      await updateTenantDept(editId.value, {
        parent_id: form.parent_id ?? 0,
        name: form.name.trim(),
        sort: form.sort,
        remark: form.remark.trim() || undefined,
      })
    } else {
      await createTenantDept({
        tenant_id: form.tenant_id!,
        parent_id: form.parent_id ?? 0,
        name: form.name.trim(),
        code: form.code.trim(),
        sort: form.sort,
        remark: form.remark.trim() || undefined,
      })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await refresh()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantDept.saveFailed'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row: DeptNode) {
  dialog.warning({
    title: t('tenantDept.deleteConfirmTitle'),
    content: t('tenantDept.deleteConfirmContent', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteTenantDept(row.id)
        message.success(t('common.deleteSuccess'))
        await refresh()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('tenantDept.deleteFailed'))
      }
    },
  })
}

// ---- 部门成员（含子部门反查；后端本地交集路径，分页结构一致） ----
const showMembers = ref(false)
const memberLoading = ref(false)
const memberRows = ref<TenantUserAccount[]>([])
const memberQuery = reactive({ tenant_id: 0, dept_id: 0, page: 1, page_size: 10 })
const memberTotal = ref(0)
const memberPagination = computed(() => ({
  page: memberQuery.page,
  pageSize: memberQuery.page_size,
  itemCount: memberTotal.value,
  onChange: (page: number) => {
    memberQuery.page = page
    loadMembers()
  },
}))

async function loadMembers() {
  memberLoading.value = true
  try {
    const { data } = await listTenantUsers({
      page: memberQuery.page,
      page_size: memberQuery.page_size,
      tenant_id: memberQuery.tenant_id,
      dept_id: memberQuery.dept_id,
    })
    memberRows.value = data.data.list
    memberTotal.value = data.data.page.total
  } finally {
    memberLoading.value = false
  }
}

function openMembers(row: DeptNode) {
  memberQuery.tenant_id = row.tenant_id
  memberQuery.dept_id = row.id
  memberQuery.page = 1
  showMembers.value = true
  loadMembers()
}

const memberColumns = computed<DataTableColumns<TenantUserAccount>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('tenantUser.username'), key: 'username', width: 130 },
  { title: t('tenantUser.nickname'), key: 'nickname', width: 110, render: (row) => row.nickname || '—' },
  {
    title: t('tenantUser.depts'), key: 'depts', width: 200,
    render: (row) => row.depts?.length
      ? h('span', { style: 'display:inline-flex;gap:4px;flex-wrap:wrap' }, row.depts.map((d) => h(NTag, { size: 'small', bordered: false }, { default: () => d.name })))
      : '—',
  },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
])

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<DeptNode>>(() => [
  { title: t('tenantDept.name'), key: 'name', minWidth: 180 },
  { title: t('tenantDept.code'), key: 'code', width: 140 },
  { title: t('tenantDept.memberCount'), key: 'member_count', width: 100 },
  { title: t('tenantDept.sort'), key: 'sort', width: 70 },
  { title: t('tenantDept.remark'), key: 'remark', width: 160, ellipsis: { tooltip: true }, render: (row) => row.remark || '—' },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 230,
    render(row) {
      const actions: TableAction[] = []
      if (userStore.has('tenantDept:create')) {
        actions.push({ label: t('tenantDept.newChild'), onClick: () => openCreate(row) })
      }
      if (userStore.has('tenantDept:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('tenantUser:list')) {
        actions.push({ label: t('tenantDept.members'), onClick: () => openMembers(row) })
      }
      if (userStore.has('tenantDept:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

// 切换租户即重载该租户部门树
watch(() => query.tenant_id, (tid) => {
  if (tid) {
    refresh()
  } else {
    all.value = []
    tree.value = []
    expandedKeys.value = []
  }
})

onMounted(() => {
  loadTenantOptions()
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
/* 展开/收起按钮固定在行最左，新增按钮靠右 */
.expand-toggle {
  margin-right: auto;
}
</style>
