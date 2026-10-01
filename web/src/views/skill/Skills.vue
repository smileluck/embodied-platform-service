<template>
  <!-- 技能管理（多文件技能包）：搜索独立卡片 + 列表；编辑弹窗内管理主指令与附属文件（二级弹窗编辑文件） -->
  <SearchCard storage-key="skills" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('skill.searchKw')" clearable style="width: 200px" @keyup.enter="search" />
    <n-select v-model:value="query.status" :options="statusOptions" :placeholder="t('common.status')" clearable style="width: 110px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['skill:create']" type="primary" ghost @click="openCreate">
          {{ t('skill.newSkill') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :pagination="pagination" remote :bordered="false" />
  </n-card>

  <!-- 新增/编辑：基本信息 + 主指令 + 附属文件管理（本地数组，保存时整体提交） -->
  <n-modal v-model:show="showModal" preset="card" :title="editing ? t('skill.editSkill') : t('skill.newSkill')" style="width: 720px" :bordered="false" size="small">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="90">
      <n-form-item :label="t('skill.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="20" show-word-limit :placeholder="t('skill.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('skill.code')" path="code">
        <n-input v-model:value="form.code" :maxlength="64" show-word-limit :placeholder="t('skill.codePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('skill.description')" path="description">
        <n-input v-model:value="form.description" :maxlength="200" show-word-limit :placeholder="t('skill.descriptionPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('skill.instruction')" path="instruction">
        <n-input
          v-model:value="form.instruction" type="textarea" :rows="10" :maxlength="8000" show-count
          :placeholder="t('skill.instructionPlaceholder')"
        />
      </n-form-item>
      <n-form-item :label="t('skill.files')">
        <div class="files-block">
          <div v-if="form.files.length === 0" class="files-empty">{{ t('skill.fileEmpty') }}</div>
          <div v-for="(f, i) in form.files" :key="i" class="file-row">
            <span class="file-path mono">{{ f.path }}</span>
            <span class="file-size">{{ t('skill.fileSize', { n: (f.content || '').length }) }}</span>
            <n-button size="tiny" quaternary @click="openFileEditor(i)">{{ t('common.edit') }}</n-button>
            <n-button size="tiny" quaternary type="error" @click="confirmRemoveFile(i)">✕</n-button>
          </div>
          <n-button size="small" dashed block :disabled="form.files.length >= 10" @click="openFileEditor(-1)">
            + {{ t('skill.addFile') }}
          </n-button>
          <div class="files-hint">{{ t('skill.filesHint') }}</div>
        </div>
      </n-form-item>
      <n-form-item :label="t('common.remark')">
        <n-input v-model:value="form.remark" :maxlength="200" show-word-limit :placeholder="t('role.remarkPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('common.status')">
        <n-switch v-model:value="form.status" :checked-value="1" :unchecked-value="0" size="small" />
      </n-form-item>
    </n-form>
    <template #action>
      <div class="modal-actions">
        <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
      </div>
    </template>
  </n-modal>

  <!-- 附属文件编辑（二级弹窗：路径 + 文本内容） -->
  <n-modal v-model:show="showFileModal" preset="card" :title="t('skill.editFile')" style="width: 640px" :bordered="false" size="small">
    <n-form label-placement="left" label-width="90">
      <n-form-item :label="t('skill.filePath')" required>
        <n-input v-model:value="fileDraft.path" :maxlength="128" :placeholder="t('skill.filePathPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('skill.fileContent')">
        <n-input v-model:value="fileDraft.content" type="textarea" :rows="14" class="mono-input" />
      </n-form-item>
    </n-form>
    <template #action>
      <div class="modal-actions">
        <n-button @click="showFileModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" @click="applyFile">{{ t('common.confirm') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NInput, NModal, NSelect, NSwitch, NTag, NTooltip,
  useDialog, useMessage, type DataTableColumns, type FormInst, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { createSkill, deleteSkill, listSkills, updateSkill } from '../../api'
import type { SkillInfo } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const query = reactive({ kw: '', status: null as number | null, page: 1, page_size: 10 })
const rows = ref<SkillInfo[]>([])
const loading = ref(false)
const { pagination, setTotal, runSearch: search } = usePagination(query, () => load())

const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 0 },
])

const columns = computed<DataTableColumns<SkillInfo>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('skill.name'), key: 'name', width: 130, ellipsis: { tooltip: true } },
  { title: t('skill.code'), key: 'code', width: 120, render: (row) => h('span', { class: 'mono' }, row.code) },
  {
    title: t('skill.description'), key: 'description', minWidth: 200,
    render: (row) => row.description
      ? h(NTooltip, null, { trigger: () => h('span', { class: 'desc-cell' }, row.description), default: () => row.description })
      : h('span', { style: 'color: var(--sx-muted)' }, '—'),
  },
  {
    title: t('skill.files'), key: 'files', width: 90,
    render: (row) => row.files?.length
      ? h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => t('skill.fileCount', { n: row.files.length }) })
      : h('span', { style: 'color: var(--sx-muted)' }, '—'),
  },
  {
    title: t('common.status'), key: 'status', width: 76,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 1 ? 'success' : 'default' },
      { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  {
    title: t('common.updateTime'), key: 'updated_at', width: 150,
    render: (row) => row.updated_at?.slice(0, 16).replace('T', ' ') ?? '—',
  },
  {
    title: t('common.actions'), key: 'actions', width: 120,
    render: (row) => {
      const actions = []
      if (userStore.has('skill:update')) actions.push({ label: t('common.edit'), onClick: () => openEdit(row) })
      if (userStore.has('skill:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
])

async function load() {
  loading.value = true
  try {
    const res = await listSkills({
      page: query.page, page_size: query.page_size,
      kw: query.kw.trim() || undefined,
      status: query.status ?? undefined,
    })
    rows.value = res.data.data.list
    setTotal(res.data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, { kw: '', status: null, page: 1 })
  load()
}

// ---- 新增/编辑 ----
const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const saving = ref(false)
const formRef = ref<FormInst>()
const form = reactive({
  name: '', code: '', description: '', instruction: '',
  files: [] as { path: string; content: string }[],
  remark: '', status: 1,
})

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('role.namePlaceholder'), trigger: ['blur', 'input'] }],
  code: [{ required: true, message: t('agent.agent.form.codeRequired'), trigger: ['blur', 'input'] }],
  instruction: [{ required: true, message: t('skill.instructionPlaceholder'), trigger: ['blur', 'input'] }],
}))

function openCreate() {
  editing.value = false
  Object.assign(form, { name: '', code: '', description: '', instruction: '', files: [], remark: '', status: 1 })
  showModal.value = true
}

function openEdit(row: SkillInfo) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, {
    name: row.name, code: row.code, description: row.description, instruction: row.instruction,
    files: (row.files ?? []).map((f) => ({ path: f.path, content: f.content })),
    remark: row.remark, status: row.status,
  })
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  // 路径重复本地预检（后端同款兜底）
  const paths = form.files.map((f) => f.path.trim()).filter(Boolean)
  if (new Set(paths).size !== paths.length) {
    message.warning(t('skill.filePathDuplicated'))
    return
  }
  saving.value = true
  try {
    const data = {
      name: form.name.trim(), code: form.code.trim(), description: form.description.trim(),
      instruction: form.instruction, remark: form.remark.trim(), status: form.status,
      files: form.files.filter((f) => f.path.trim() !== ''),
    }
    if (editing.value) {
      await updateSkill(editId.value, data)
    } else {
      await createSkill(data)
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

function confirmDelete(row: SkillInfo) {
  dialog.warning({
    title: t('common.tips'),
    content: t('skill.deleteConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteSkill(row.id)
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

// ---- 附属文件编辑（二级弹窗；fileIndex=-1 新增） ----
const showFileModal = ref(false)
const fileIndex = ref(-1)
const fileDraft = reactive({ path: '', content: '' })

function openFileEditor(index: number) {
  fileIndex.value = index
  if (index >= 0) {
    Object.assign(fileDraft, { path: form.files[index].path, content: form.files[index].content })
  } else {
    Object.assign(fileDraft, { path: '', content: '' })
  }
  showFileModal.value = true
}

function applyFile() {
  const path = fileDraft.path.trim()
  if (!path) {
    message.warning(t('skill.filePathPlaceholder'))
    return
  }
  const dup = form.files.some((f, i) => i !== fileIndex.value && f.path.trim() === path)
  if (dup) {
    message.warning(t('skill.filePathDuplicated'))
    return
  }
  if (fileIndex.value >= 0) {
    form.files[fileIndex.value] = { path, content: fileDraft.content }
  } else {
    form.files.push({ path, content: fileDraft.content })
  }
  showFileModal.value = false
}

function confirmRemoveFile(index: number) {
  dialog.warning({
    title: t('common.tips'),
    content: t('skill.fileDeleteConfirm', { path: form.files[index].path }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => { form.files.splice(index, 1) },
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
.mono {
  font-family: var(--sx-font-mono);
}
.desc-cell {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
}

/* 附属文件列表 */
.files-block {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.files-empty {
  color: var(--sx-muted);
  font-size: 12px;
}
.file-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 5px 10px;
  border-radius: 6px;
  background: var(--sx-bg);
  border: 1px solid var(--sx-line);
  font-size: 12px;
}
.file-path {
  color: var(--sx-accent);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-size {
  color: var(--sx-muted);
  flex-shrink: 0;
}
.files-hint {
  font-size: 12px;
  color: var(--sx-muted);
}

/* 文件内容编辑器用等宽字体 */
.mono-input :deep(textarea) {
  font-family: var(--sx-font-mono);
  font-size: 12px;
}
</style>
