<template>
  <!-- 数据字典：左右 4:6 分栏，左侧字典类型，右侧选中类型的字典项 -->
  <div class="dict-page">
    <div class="cols">
      <!-- 左栏：类型列表 -->
      <div class="col col-left">
        <n-card size="small" class="type-card">
          <template #header>
            <span class="card-title">{{ t('dict.type.title') }}</span>
          </template>
          <template #header-extra>
            <n-button v-permission="['dict:type:create']" size="small" type="primary" ghost @click="openTypeCreate">
              {{ t('dict.type.new') }}
            </n-button>
          </template>
          <SearchCard storage-key="dictTypes" @search="loadTypes(1)" @reset="resetQuery">
            <n-input v-model:value="query.name" size="small" :placeholder="t('dict.type.name')" clearable @keyup.enter="loadTypes(1)" />
            <n-input v-model:value="query.code" size="small" :placeholder="t('dict.type.code')" clearable @keyup.enter="loadTypes(1)" />
          </SearchCard>
          <div v-if="typesLoading" style="padding: 24px 0; text-align: center"><n-spin size="small" /></div>
          <n-empty v-else-if="!types.length" size="small" :description="t('common.noData')" style="padding: 24px 0" />
          <div v-else class="type-list">
            <div
              v-for="ty in types" :key="ty.id"
              class="type-item" :class="{ active: selectedType?.id === ty.id }"
              @click="selectType(ty)"
            >
              <div class="type-row">
                <span class="type-name">{{ ty.name }}</span>
                <n-tag size="small" :type="ty.status === 1 ? 'success' : 'default'" :bordered="false">
                  {{ ty.status === 1 ? t('common.enabled') : t('common.disabled') }}
                </n-tag>
              </div>
              <div class="type-code mono">{{ ty.code }}</div>
              <div class="type-ops">
                <n-button v-if="userStore.has('dict:type:update')" size="tiny" quaternary @click.stop="openTypeEdit(ty)">{{ t('common.edit') }}</n-button>
                <n-button v-if="userStore.has('dict:type:delete')" size="tiny" quaternary type="error" @click.stop="confirmTypeDelete(ty)">✕</n-button>
              </div>
            </div>
          </div>
          <div class="type-pager">
            <n-pagination size="small" :page="typePage" :page-size="20" :item-count="typeTotal" @update:page="loadTypes" />
          </div>
        </n-card>
      </div>

      <!-- 右栏：选中类型的字典项 -->
      <div class="col col-right">
        <n-card size="small" class="item-card">
          <template #header>
            <span class="card-title">
              {{ selectedType ? `${t('dict.item.title')} · ${selectedType.name}` : t('dict.item.title') }}
            </span>
          </template>
          <template #header-extra>
            <n-button
              v-if="selectedType" v-permission="['dict:item:create']"
              size="small" type="primary" ghost @click="openItemCreate"
            >{{ t('dict.item.new') }}</n-button>
          </template>
          <n-empty v-if="!selectedType" size="small" :description="t('dict.type.selectHint')" style="padding: 60px 0" />
          <n-data-table v-else size="small" :columns="itemColumns" :data="items" :loading="itemsLoading" :bordered="false" />
        </n-card>
      </div>
    </div>

    <!-- 类型编辑 -->
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

    <!-- 字典项编辑 -->
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
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NEmpty, NForm, NFormItem, NInput, NInputNumber, NModal,
  NPagination, NSpin, NSwitch, NTag, useDialog, useMessage,
  type DataTableColumns, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { useUserStore } from '../../stores/user'
import { createDictItem, createDictType, deleteDictItem, deleteDictType, listDictItems, listDictTypes, updateDictItem, updateDictType } from '../../api'
import type { DictItem, DictType } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

// ---- 左栏：类型 ----
const query = reactive({ name: '', code: '' })
const types = ref<DictType[]>([])
const typePage = ref(1)
const typeTotal = ref(0)
const typesLoading = ref(false)
const selectedType = ref<DictType | null>(null)

async function loadTypes(page = 1) {
  typesLoading.value = true
  try {
    const res = await listDictTypes({ page, page_size: 20, name: query.name || undefined, code: query.code || undefined })
    types.value = res.data.data.list
    typeTotal.value = res.data.data.page.total
    typePage.value = page
    if (!types.value.find((ty) => ty.id === selectedType.value?.id)) {
      selectedType.value = types.value[0] ?? null
      if (selectedType.value) loadItems()
    }
  } finally {
    typesLoading.value = false
  }
}

function resetQuery() {
  query.name = ''
  query.code = ''
  loadTypes(1)
}

function selectType(ty: DictType) {
  selectedType.value = ty
  loadItems()
}

// ---- 右栏：字典项 ----
const items = ref<DictItem[]>([])
const itemsLoading = ref(false)

async function loadItems() {
  if (!selectedType.value) return
  itemsLoading.value = true
  try {
    // 字典项数量有限，一页取全（page_size 上限内）
    const res = await listDictItems(selectedType.value.id, { page: 1, page_size: 0 })
    items.value = res.data.data.list
  } finally {
    itemsLoading.value = false
  }
}

const itemColumns = computed<DataTableColumns<DictItem>>(() => {
  const cols: DataTableColumns<DictItem> = [
    { title: t('dict.item.label'), key: 'label' },
    { title: t('dict.item.value'), key: 'value', className: 'mono' },
    { title: t('dict.item.sort'), key: 'sort', width: 70 },
    {
      title: t('common.status'), key: 'status', width: 80,
      render: (row) => h(NTag, { size: 'small', type: row.status === 1 ? 'success' : 'default', bordered: false },
        { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
    },
    { title: t('common.remark'), key: 'remark', ellipsis: { tooltip: true } },
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

// ---- 类型表单 ----
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
    loadTypes(typePage.value)
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
        loadTypes(typePage.value)
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

// 类型行操作（挂在类型项 hover？保持简单：选中后在右栏头部不合适——放左栏项内右侧小按钮）
// 采用：类型项点击选中；编辑/删除通过右键不友好，故在选中项下方提供操作（见模板 type-item active 状态渲染按钮）
// 简化实现：左栏每项尾部放编辑/删除小按钮（hover 显示）

// ---- 字典项表单 ----
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
  if (!selectedType.value) return
  saving.value = true
  try {
    if (itemEditing.value) {
      await updateDictItem(itemEditId.value, { ...itemForm })
    } else {
      await createDictItem(selectedType.value.id, { ...itemForm })
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
.dict-page {
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.cols {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.col {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.col-left {
  flex: 4 1 0;
}
.col-right {
  flex: 6 1 0;
}
.card-title {
  font-weight: 600;
}
.type-card :deep(.n-card__content) {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.type-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 480px;
  overflow: auto;
}
.type-item {
  padding: 8px 10px;
  border: 1px solid var(--sx-line);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}
.type-item:hover {
  border-color: var(--sx-accent);
}
.type-item.active {
  border-color: var(--sx-accent);
  background-color: rgba(63, 117, 171, 0.08);
}
.type-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.type-name {
  font-weight: 600;
}
.type-code {
  margin-top: 2px;
  font-size: 12px;
  color: var(--sx-muted);
}
.type-ops {
  position: absolute;
  top: 50%;
  right: 6px;
  transform: translateY(-50%);
  display: none;
  gap: 2px;
}
.type-item {
  position: relative;
}
.type-item:hover .type-ops {
  display: flex;
}
.type-pager {
  display: flex;
  justify-content: flex-end;
}
.mono {
  font-family: var(--sx-font-mono);
}
</style>
