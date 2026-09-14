<template>
  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button class="expand-toggle" @click="toggleExpand">{{ expandAll ? t('permMenu.collapseAll') : t('permMenu.expandAll') }}</n-button>
        <n-button type="primary" ghost @click="openCreate(0, 'dir')" v-permission="['menu:create']">{{ t('permMenu.newTopDir') }}</n-button>
        <n-button type="primary" ghost @click="openCreate(0, 'menu')" v-permission="['menu:create']">{{ t('permMenu.newTopMenu') }}</n-button>
        <n-button type="primary" ghost @click="openCreate(0, 'button')" v-permission="['menu:create']">{{ t('permMenu.newButton') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="tree" :loading="loading" :row-key="rowKey"
      :expanded-row-keys="expandedKeys" @update:expanded-row-keys="onExpandUpdate" />
  </n-card>

  <!-- 目录 / 菜单 / 权限点编辑：type 区分表单 -->
  <n-modal v-model:show="showModal" preset="dialog" :title="modalTitle" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('permMenu.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="20" show-word-limit :placeholder="t('permMenu.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('permMenu.code')" path="code" v-if="!editing">
        <n-input v-model:value="form.code" :maxlength="64" show-word-limit :placeholder="t('permMenu.codePlaceholder', { type: form.type })" />
      </n-form-item>
      <!-- 目录：顶级分组，无路由无父级，仅图标/排序 -->
      <template v-if="form.type === 'dir'">
        <n-form-item :label="t('permMenu.icon')">
          <n-input-group>
            <n-input v-model:value="form.icon" :placeholder="t('permMenu.iconPlaceholder')">
              <template #prefix>
                <IconPreview v-if="form.icon" :icon="form.icon" :size="16" />
              </template>
            </n-input>
            <n-button @click="openPicker">{{ t('permMenu.pickIcon') }}</n-button>
          </n-input-group>
        </n-form-item>
      </template>
      <!-- 菜单：页面，父级仅可挂目录（或留空为顶级） -->
      <template v-else-if="form.type === 'menu'">
        <n-form-item :label="t('permMenu.route')"><n-input v-model:value="form.path" :placeholder="t('permMenu.routePlaceholder')" /></n-form-item>
        <n-form-item :label="t('permMenu.parent')">
          <n-tree-select
            v-model:value="form.parent_id" :options="parentOptions"
            clearable :placeholder="t('permMenu.parentPlaceholderMenu')" key-field="key" label-field="label" children-field="children"
          />
        </n-form-item>
        <n-form-item :label="t('permMenu.icon')">
          <n-input-group>
            <n-input v-model:value="form.icon" :placeholder="t('permMenu.iconPlaceholder')">
              <template #prefix>
                <IconPreview v-if="form.icon" :icon="form.icon" :size="16" />
              </template>
            </n-input>
            <n-button @click="openPicker">{{ t('permMenu.pickIcon') }}</n-button>
          </n-input-group>
        </n-form-item>
      </template>
      <!-- 权限点：父级仅可挂菜单 -->
      <template v-else>
        <n-form-item label="Method">
          <n-select v-model:value="form.method" :options="methodOptions" />
        </n-form-item>
        <n-form-item :label="t('permMenu.apiPath')">
          <n-input v-model:value="form.path" :placeholder="t('permMenu.apiPathPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('permMenu.parentMenu')">
          <n-tree-select
            v-model:value="form.parent_id" :options="parentOptions"
            clearable :placeholder="t('permMenu.parentPlaceholderButton')" key-field="key" label-field="label" children-field="children"
          />
        </n-form-item>
      </template>
      <n-form-item :label="t('permMenu.sort')"><n-input-number v-model:value="form.sort" /></n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 图标选择器：本地图标搜索 + 自定义图片 URL -->
  <n-modal v-model:show="showPicker" preset="card" :title="t('permMenu.pickIcon')" style="width: 640px">
    <n-tabs type="line" size="small">
      <n-tab-pane name="local" :tab="t('permMenu.localIcons')">
        <n-input v-model:value="iconSearch" :placeholder="t('permMenu.iconSearchPlaceholder')" clearable style="margin-bottom: 12px" />
        <div class="icon-grid">
          <div v-for="name in filteredIconNames" :key="name" class="icon-cell" :title="name"
            :class="{ active: form.icon === name }" @click="pickIcon(name)">
            <component :is="(icons as any)[name]" />
            <span class="icon-cell-name">{{ name }}</span>
          </div>
        </div>
      </n-tab-pane>
      <n-tab-pane name="url" :tab="t('permMenu.webImage')">
        <n-space vertical>
          <n-input v-model:value="iconUrl" placeholder="https://example.com/icon.png" clearable />
          <div class="url-preview">
            <span>{{ t('permMenu.preview') }}</span>
            <IconPreview v-if="iconUrl" :icon="iconUrl" :size="20" />
            <span v-else>—</span>
          </div>
          <n-button type="primary" :disabled="!iconUrl" @click="pickIcon(iconUrl)">{{ t('permMenu.useThisImage') }}</n-button>
        </n-space>
      </n-tab-pane>
    </n-tabs>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, type VNode } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NButton, NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber, NSelect, NInputGroup, NTabs, NTabPane, NTreeSelect, NTag, useMessage, useDialog, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import * as icons from '@vicons/ionicons5'
import { createPermission, deletePermission, listAllPermissions, updatePermission } from '../../api'
import { renderMenuIcon } from '../../utils/menuIcon'
import { useUserStore } from '../../stores/user'
import type { Permission } from '../../api/types'

// 模板中预览用的函数式组件
const IconPreview = (props: { icon?: string; size?: number }) => renderMenuIcon(props.icon, props.size ?? 16)

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()
const loading = ref(false)
const all = ref<Permission[]>([])
const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const formRef = ref<FormInst | null>(null)

// 超管通配权限（id=1，code=all）禁止删除
const WILDCARD_PERM_ID = 1

// form.type 区分目录 / 菜单 / 按钮权限点三种表单（dir → menu → button 三级模型）
const form = reactive({
  name: '', code: '', type: 'menu' as 'dir' | 'menu' | 'button',
  method: 'GET', path: '', parent_id: null as number | null, icon: '', sort: 0,
})
const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', '*'].map((m) => ({ label: m, value: m }))
const typeNames = computed(() => ({
  dir: t('permMenu.typeDir'), menu: t('permMenu.typeMenu'), button: t('permMenu.typeButton'),
}))
const modalTitle = computed(() => `${editing.value ? t('common.edit') : t('common.add')} ${typeNames.value[form.type]}`)

const rules = computed<FormRules>(() => ({
  name: [
    { required: true, message: t('permMenu.form.nameRequired'), trigger: ['blur', 'input'] },
    { max: 20, message: t('permMenu.form.nameMax'), trigger: ['blur', 'input'] },
  ],
  code: [
    { required: true, message: t('permMenu.form.codeRequired'), trigger: ['blur', 'input'] },
    { max: 64, message: t('permMenu.form.codeMax'), trigger: ['blur', 'input'] },
  ],
}))

// 图标选择器
const showPicker = ref(false)
const iconSearch = ref('')
const iconUrl = ref('')
const iconNames = Object.keys(icons).filter((k) => k !== 'default')
const filteredIconNames = computed(() => {
  const kw = iconSearch.value.trim().toLowerCase()
  const list = kw ? iconNames.filter((n) => n.toLowerCase().includes(kw)) : iconNames
  return list.slice(0, 200) // 避免一次渲染上千节点
})

function openPicker() {
  iconSearch.value = ''
  iconUrl.value = ''
  showPicker.value = true
}

function pickIcon(v: string) {
  form.icon = v
  showPicker.value = false
}

async function load() {
  loading.value = true
  try {
    // 树目录需整表构建，走全量接口（page_size=0），分页会截断子节点
    const { data } = await listAllPermissions()
    all.value = data.data.list
  } finally {
    loading.value = false
  }
}

// 平铺 -> 树（菜单 + 按钮权限点统一按 parent_id 组树，n-data-table rowKey/children）
const tree = ref<any[]>([])
function buildTree() {
  const build = (parentID: number): any[] =>
    all.value
      .filter((p) => p.parent_id === parentID)
      .sort((a, b) => a.sort - b.sort)
      .map((p) => {
        const children = build(p.id)
        const n: any = { id: p.id, name: p.name, code: p.code, type: p.type, method: p.method, path: p.path, icon: p.icon, sort: p.sort }
        if (children.length) n.children = children
        return n
      })
  tree.value = build(0)
}

// 树表受控展开：default-expand-all 对异步加载后才渲染的行不生效，改为手动维护展开行键
const rowKey = (row: any) => row.id
const expandedKeys = ref<number[]>([])
const expandAll = ref(true)
function collectParentIds(nodes: any[]): number[] {
  return nodes.flatMap((n) => (n.children?.length ? [n.id, ...collectParentIds(n.children)] : []))
}
function onExpandUpdate(keys: Array<string | number>) {
  expandedKeys.value = keys as number[]
}
function toggleExpand() {
  expandAll.value = !expandAll.value
  expandedKeys.value = expandAll.value ? collectParentIds(tree.value) : []
}

// 父级选项：dir → menu 层级组树，仅目标类型可选（菜单的父级仅目录、权限点的父级仅菜单；
// 其余节点禁用仅作层级展示，无需再排除自身——可选类型与被编辑节点类型必然不同）
const parentOptions = ref<any[]>([])
function buildParentOptions(selectableType: 'dir' | 'menu') {
  const nodes = all.value.filter((p) => p.type === 'dir' || p.type === 'menu')
  const build = (parentID: number): any[] =>
    nodes
      .filter((p) => p.parent_id === parentID)
      .sort((a, b) => a.sort - b.sort)
      .map((p) => {
        const children = build(p.id)
        const n: any = { label: p.name, key: p.id, disabled: p.type !== selectableType }
        if (children.length) n.children = children
        return n
      })
  parentOptions.value = build(0)
}

function openCreate(parentID: number, type: 'dir' | 'menu' | 'button') {
  editing.value = false
  Object.assign(form, { name: '', code: '', type, method: 'GET', path: '', parent_id: parentID || null, icon: '', sort: 0 })
  if (type !== 'dir') buildParentOptions(type === 'menu' ? 'dir' : 'menu')
  showModal.value = true
}

function openEdit(row: any) {
  editing.value = true
  editId.value = row.id
  const p = all.value.find((x) => x.id === row.id)!
  Object.assign(form, {
    name: p.name, code: p.code, type: p.type as 'dir' | 'menu' | 'button',
    method: p.method || 'GET', path: p.path, parent_id: p.parent_id || null, icon: p.icon, sort: p.sort,
  })
  if (p.type !== 'dir') buildParentOptions(p.type === 'menu' ? 'dir' : 'menu')
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return // 校验失败，错误已在表单项上展示
  }
  try {
    if (editing.value) {
      await updatePermission(editId.value, {
        name: form.name.trim(),
        method: form.type === 'button' ? form.method : '',
        path: form.path.trim(),
        icon: form.type !== 'button' ? form.icon : '',
        sort: form.sort,
        parent_id: form.type === 'dir' ? 0 : (form.parent_id ?? 0),
      })
    } else {
      await createPermission({
        name: form.name.trim(), code: form.code.trim(), type: form.type,
        method: form.type === 'button' ? form.method : '',
        path: form.type === 'dir' ? '' : form.path.trim(),
        parent_id: form.type === 'dir' ? 0 : (form.parent_id ?? 0),
        icon: form.type !== 'button' ? form.icon : '', sort: form.sort,
      })
    }
    message.success(t('permMenu.saveSuccessNote'))
    showModal.value = false
    await refresh()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('permMenu.saveFailed'))
  }
}

async function remove(row: any) {
  if (row.id === WILDCARD_PERM_ID) { message.error(t('permMenu.wildcardForbidden')); return }
  if (all.value.some((p) => p.parent_id === row.id)) {
    message.warning(t('permMenu.hasChildren'))
    return
  }
  dialog.warning({
    title: t('permMenu.deleteConfirmTitle'),
    content: t('permMenu.deleteConfirmContent', { name: row.name, code: row.code }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deletePermission(row.id)
        message.success(t('common.deleteSuccess'))
        await refresh()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('permMenu.deleteFailed'))
      }
    },
  })
}

async function refresh() {
  await load()
  buildTree()
  // 数据刷新后按当前展开状态重算行键，保证新增的父节点也默认展开
  if (expandAll.value) expandedKeys.value = collectParentIds(tree.value)
}

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<any>>(() => [
  {
    title: t('permMenu.name'), key: 'name',
    render(row) {
      return h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, [
        row.type !== 'button' ? renderMenuIcon(row.icon, 16) : null,
        h('span', {}, row.name),
      ])
    },
  },
  {
    title: t('permMenu.type'), key: 'type', width: 80,
    render(row) {
      const tagType = row.type === 'dir' ? 'warning' : row.type === 'menu' ? 'success' : 'info'
      return h(NTag, { size: 'small', type: tagType }, { default: () => typeNames.value[row.type as keyof typeof typeNames.value] || row.type })
    },
  },
  { title: t('permMenu.code'), key: 'code' },
  {
    title: t('permMenu.routeApi'), key: 'path',
    render(row) {
      if (row.type === 'dir') return '—'
      if (row.type === 'menu') return row.path || '—'
      return row.method && row.path ? `${row.method}  ${row.path}` : '—'
    },
  },
  { title: t('permMenu.sort'), key: 'sort', width: 70 },
  {
    title: t('common.operation'), key: 'actions', width: 210,
    render(row) {
      const actions: Array<TableAction | VNode> = []
      if (userStore.has('menu:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      // 目录下加菜单、菜单下加权限点（dir → menu → button 三级模型）
      if (row.type === 'dir' && userStore.has('menu:create')) {
        actions.push({ label: t('permMenu.addSubMenu'), onClick: () => openCreate(row.id, 'menu') })
      }
      if (row.type === 'menu' && userStore.has('menu:create')) {
        actions.push({ label: t('permMenu.addPermPoint'), onClick: () => openCreate(row.id, 'button') })
      }
      // 超管通配权限（all）禁止删除，不展示删除按钮
      if (row.id !== WILDCARD_PERM_ID && userStore.has('menu:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => remove(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(refresh)
</script>

<style scoped>
/* 卡头只放操作按钮（页面标题由顶栏展示） */
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
/* 展开/收起按钮固定在行最左，其余新增按钮靠右 */
.expand-toggle {
  margin-right: auto;
}
.icon-grid {
  max-height: 360px;
  overflow: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
  gap: 6px;
}
.icon-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 8px 4px;
  border: 1px solid var(--sx-line, #e0e0e0);
  border-radius: 6px;
  cursor: pointer;
  font-size: 18px;
  transition: border-color 0.15s ease, background 0.15s ease;
}
.icon-cell:hover {
  border-color: var(--sx-accent, #3F75AB);
}
.icon-cell.active {
  border-color: var(--sx-accent, #3F75AB);
  background: var(--sx-accent-soft, #E4EDF6);
}
.icon-cell :deep(svg) {
  width: 20px;
  height: 20px;
}
.icon-cell-name {
  font-size: 10px;
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.url-preview {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
}
</style>
