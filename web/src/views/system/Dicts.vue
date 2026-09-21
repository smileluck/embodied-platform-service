<template>
  <!-- 数据字典：整页字典类型列表；「字典项」点击后弹出右侧抽屉维护 -->
  <SearchCard storage-key="dictTypes" @search="loadTypes(1)" @reset="resetQuery">
    <n-input v-model:value="query.name" :placeholder="t('dict.type.name')" clearable style="width: 200px" @keyup.enter="loadTypes(1)" />
    <n-input v-model:value="query.code" :placeholder="t('dict.type.code')" clearable style="width: 200px" @keyup.enter="loadTypes(1)" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['dict:type:create']" type="primary" ghost @click="openTypeCreate">
          {{ t('dict.type.new') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="typeColumns" :data="types" :loading="typesLoading" :pagination="pagination" remote :bordered="false" />
  </n-card>

  <!-- 类型新增/编辑 -->
  <n-modal v-model:show="showTypeModal" preset="dialog" :title="typeEditing ? t('dict.type.edit') : t('dict.type.new')" style="width: 460px">
    <n-form :model="typeForm" :rules="typeRules" label-placement="left" label-width="80">
      <n-form-item :label="t('dict.type.name')" path="name">
        <n-input v-model:value="typeForm.name" :maxlength="20" show-count />
      </n-form-item>
      <n-form-item :label="t('dict.type.code')" path="code">
        <n-input v-model:value="typeForm.code" :maxlength="64" show-count placeholder="user_status" />
      </n-form-item>
      <n-form-item :label="t('common.remark')">
        <n-input v-model:value="typeForm.remark" :maxlength="200" />
      </n-form-item>
      <n-form-item :label="t('common.status')">
        <n-switch v-model:value="typeForm.status" :checked-value="1" :unchecked-value="0" size="small" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showTypeModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="saveType">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 字典项抽屉：展示/维护选中类型的字典项 -->
  <n-drawer v-model:show="showItems" :width="680" :auto-focus="false" placement="right">
    <n-drawer-content closable>
      <template #header>
        <div class="items-header">
          <span class="items-title">{{ t('dict.item.title') }} · {{ itemsType?.name }}</span>
          <n-button
            v-if="itemsType" v-permission="['dict:item:create']"
            size="small" type="primary" ghost @click="openItemCreate"
          >{{ t('dict.item.new') }}</n-button>
        </div>
      </template>
      <div class="items-type-code mono">{{ itemsType?.code }}</div>
      <n-data-table size="small" :columns="itemColumns" :data="items" :loading="itemsLoading" :bordered="false" />
    </n-drawer-content>
  </n-drawer>

  <!-- 字典项新增/编辑 -->
  <n-modal v-model:show="showItemModal" preset="dialog" :title="itemEditing ? t('dict.item.edit') : t('dict.item.new')" style="width: 460px">
    <n-form :model="itemForm" :rules="itemRules" label-placement="left" label-width="80">
      <n-form-item :label="t('dict.item.label')" path="label">
        <n-input v-model:value="itemForm.label" :maxlength="20" show-count />
      </n-form-item>
      <n-form-item :label="t('dict.item.value')" path="value">
        <n-input v-model:value="itemForm.value" :maxlength="64" show-count />
      </n-form-item>
      <n-form-item :label="t('dict.item.sort')">
        <n-input-number v-model:value="itemForm.sort" :min="0" :max="9999" style="width: 100%" />
      </n-form-item>
      <n-form-item :label="t('common.remark')">
        <n-input v-model:value="itemForm.remark" :maxlength="200" />
      </n-form-item>
      <n-form-item :label="t('common.status')">
        <n-switch v-model:value="itemForm.status" :checked-value="1" :unchecked-value="0" size="small" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showItemModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="saveItem">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NInput, NInputNumber, NModal,
  NDrawer, NDrawerContent, NSwitch, NTag, useDialog, useMessage,
  type DataTableColumns, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { createDictItem, createDictType, deleteDictItem, deleteDictType, listDictItems, listDictTypes, updateDictItem, updateDictType } from '../../api'
import type { DictItem, DictType } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

// ---- 字典类型列表 ----
const query = reactive({ name: '', code: '', page: 1, page_size: 20 })
const types = ref<DictType[]>([])
const typesLoading = ref(false)
const { pagination, setTotal } = usePagination(query, () => loadTypes(query.page))

async function loadTypes(page = 1) {
  typesLoading.value = true
  try {
    query.page = page
    const res = await listDictTypes({ page, page_size: query.page_size, name: query.name || undefined, code: query.code || undefined })
    types.value = res.data.data.list
    setTotal(res.data.data.page.total)
  } finally {
    typesLoading.value = false
  }
}

function resetQuery() {
  query.name = ''
  query.code = ''
  loadTypes(1)
}

const typeColumns = computed<DataTableColumns<DictType>>(() => [
  { title: t('dict.type.name'), key: 'name', minWidth: 160 },
  { title: t('dict.type.code'), key: 'code', className: 'mono', minWidth: 160 },
  {
    title: t('common.status'), key: 'status', width: 90,
    render: (row) => h(NTag, { size: 'small', type: row.status === 1 ? 'success' : 'default', bordered: false },
      { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.remark'), key: 'remark', minWidth: 140, ellipsis: { tooltip: true }, render: (row) => row.remark || '—' },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => {
      const actions: TableAction[] = []
      if (userStore.has('dict:item:list')) actions.push({ label: t('dict.item.title'), onClick: () => openItems(row) })
      if (userStore.has('dict:type:update')) actions.push({ label: t('common.edit'), onClick: () => openTypeEdit(row) })
      if (userStore.has('dict:type:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmTypeDelete(row) })
      return renderActions(actions)
    },
  },
])

// ---- 类型新增/编辑 ----
const showTypeModal = ref(false)
const typeEditing = ref(false)
const typeEditId = ref(0)
const typeForm = reactive({ name: '', code: '', remark: '', status: 1 })
const typeRules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('dict.type.nameRequired'), trigger: ['blur', 'change'] }],
  code: [{ required: true, message: t('dict.type.codeRequired'), trigger: ['blur', 'change'] }],
}))

function openTypeCreate() {
  typeEditing.value = false
  Object.assign(typeForm, { name: '', code: '', remark: '', status: 1 })
  showTypeModal.value = true
}

function openTypeEdit(ty: DictType) {
  typeEditing.value = true
  typeEditId.value = ty.id
  Object.assign(typeForm, { name: ty.name, code: ty.code, remark: ty.remark, status: ty.status })
  showTypeModal.value = true
}

const saving = ref(false)

async function saveType() {
  saving.value = true
  try {
    if (typeEditing.value) {
      await updateDictType(typeEditId.value, { ...typeForm })
    } else {
      await createDictType({ ...typeForm })
    }
    message.success(t('common.success'))
    showTypeModal.value = false
    loadTypes(query.page)
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    saving.value = false
  }
}

function confirmTypeDelete(ty: DictType) {
  dialog.warning({
    title: t('common.tips'),
    content: t('dict.type.deleteConfirm', { name: ty.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteDictType(ty.id)
        loadTypes(query.page)
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

// ---- 字典项抽屉 ----
const showItems = ref(false)
const itemsType = ref<DictType | null>(null)
const items = ref<DictItem[]>([])
const itemsLoading = ref(false)

function openItems(ty: DictType) {
  itemsType.value = ty
  showItems.value = true
  loadItems()
}

async function loadItems() {
  if (!itemsType.value) return
  itemsLoading.value = true
  try {
    // 字典项数量有限，一页取全（page_size 上限内）
    const res = await listDictItems(itemsType.value.id, { page: 1, page_size: 0 })
    items.value = res.data.data.list
  } finally {
    itemsLoading.value = false
  }
}

const itemColumns = computed<DataTableColumns<DictItem>>(() => {
  const cols: DataTableColumns<DictItem> = [
    { title: t('dict.item.label'), key: 'label', minWidth: 120 },
    { title: t('dict.item.value'), key: 'value', className: 'mono', minWidth: 120 },
    { title: t('dict.item.sort'), key: 'sort', width: 70 },
    {
      title: t('common.status'), key: 'status', width: 80,
      render: (row) => h(NTag, { size: 'small', type: row.status === 1 ? 'success' : 'default', bordered: false },
        { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
    },
    { title: t('common.remark'), key: 'remark', ellipsis: { tooltip: true }, render: (row) => row.remark || '—' },
  ]
  cols.push({
    title: t('common.actions'), key: 'actions', width: 110,
    render: (row) => {
      const actions: TableAction[] = []
      if (userStore.has('dict:item:update')) actions.push({ label: t('common.edit'), onClick: () => openItemEdit(row) })
      if (userStore.has('dict:item:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmItemDelete(row) })
      return renderActions(actions)
    },
  })
  return cols
})

// ---- 字典项新增/编辑 ----
const showItemModal = ref(false)
const itemEditing = ref(false)
const itemEditId = ref(0)
const itemForm = reactive({ label: '', value: '', sort: 0, remark: '', status: 1 })
const itemRules = computed<FormRules>(() => ({
  label: [{ required: true, message: t('dict.item.labelRequired'), trigger: ['blur', 'change'] }],
  value: [{ required: true, message: t('dict.item.valueRequired'), trigger: ['blur', 'change'] }],
}))

function openItemCreate() {
  itemEditing.value = false
  Object.assign(itemForm, { label: '', value: '', sort: (items.value.length + 1) * 10, remark: '', status: 1 })
  showItemModal.value = true
}

function openItemEdit(row: DictItem) {
  itemEditing.value = true
  itemEditId.value = row.id
  Object.assign(itemForm, { label: row.label, value: row.value, sort: row.sort, remark: row.remark, status: row.status })
  showItemModal.value = true
}

async function saveItem() {
  if (!itemsType.value) return
  saving.value = true
  try {
    if (itemEditing.value) {
      await updateDictItem(itemEditId.value, { ...itemForm })
    } else {
      await createDictItem(itemsType.value.id, { ...itemForm })
    }
    message.success(t('common.success'))
    showItemModal.value = false
    loadItems()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    saving.value = false
  }
}

function confirmItemDelete(row: DictItem) {
  dialog.warning({
    title: t('common.tips'),
    content: t('dict.item.deleteConfirm', { label: row.label }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteDictItem(row.id)
        loadItems()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

onMounted(() => loadTypes(1))
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}
.items-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.items-title {
  font-weight: 600;
}
.items-type-code {
  font-size: 12px;
  color: var(--sx-muted);
  margin-bottom: 10px;
}
.mono {
  font-family: var(--sx-font-mono);
}
</style>
