<template>
  <!-- 系统参数：键值列表 + 行内编辑值；键与类型创建后不可变 -->
  <SearchCard storage-key="sysConfigs" @search="load" @reset="kw = ''">
    <n-input v-model:value="kw" :placeholder="t('sysconfig.keyword')" clearable style="width: 220px" @keyup.enter="load" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['sysconfig:create']" type="primary" ghost @click="openCreate">
          {{ t('sysconfig.new') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :bordered="false" />
  </n-card>

  <n-modal v-model:show="showModal" preset="dialog" :title="t('sysconfig.new')" style="width: 480px">
    <n-form :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('sysconfig.key')" path="key">
        <n-input v-model:value="form.key" :maxlength="64" show-count :placeholder="t('sysconfig.keyPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('sysconfig.type')" path="type">
        <n-radio-group v-model:value="form.type" size="small">
          <n-radio-button value="string">string</n-radio-button>
          <n-radio-button value="number">number</n-radio-button>
          <n-radio-button value="bool">bool</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-form-item :label="t('sysconfig.value')" path="value">
        <n-input v-model:value="form.value" :maxlength="512" />
      </n-form-item>
      <n-form-item :label="t('sysconfig.description')">
        <n-input v-model:value="form.description" :maxlength="200" />
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
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NInput, NModal, NRadioButton, NRadioGroup,
  NTag, useDialog, useMessage, type DataTableColumns, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { useUserStore } from '../../stores/user'
import { createSysConfig, deleteSysConfig, listSysConfigs, updateSysConfig } from '../../api'
import type { SysConfig } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const kw = ref('')
const rows = ref<SysConfig[]>([])
const loading = ref(false)
const saving = ref(false)
const showModal = ref(false)
const form = reactive<{ key: string; value: string; type: 'string' | 'number' | 'bool'; description: string }>({ key: '', value: '', type: 'string', description: '' })

const rules = computed<FormRules>(() => ({
  key: [{ required: true, message: t('sysconfig.keyRequired'), trigger: ['blur', 'change'] }],
  value: [{ required: true, message: t('sysconfig.valueRequired'), trigger: ['blur', 'change'] }],
}))

// 行内编辑态
const editingKey = ref('')
const editValue = ref('')

const columns = computed<DataTableColumns<SysConfig>>(() => [
  { title: t('sysconfig.key'), key: 'key', width: 220, className: 'mono' },
  {
    title: t('sysconfig.value'), key: 'value',
    render: (row) => {
      if (editingKey.value !== row.key) {
        return h('span', { class: 'mono' }, row.value)
      }
      return h(NInput, {
        value: editValue.value, size: 'tiny', 'onUpdate:value': (v: string) => (editValue.value = v),
        style: 'width: 240px',
      })
    },
  },
  {
    title: t('sysconfig.type'), key: 'type', width: 90,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => row.type }),
  },
  { title: t('sysconfig.description'), key: 'description', ellipsis: { tooltip: true } },
  {
    title: t('common.actions'), key: 'actions', width: 170,
    render: (row) => {
      const btns = []
      if (editingKey.value === row.key) {
        btns.push({ label: t('common.save'), onClick: () => saveEdit(row) })
        btns.push({ label: t('common.cancel'), onClick: () => (editingKey.value = '') })
      } else if (userStore.has('sysconfig:update')) {
        btns.push({ label: t('common.edit'), onClick: () => { editingKey.value = row.key; editValue.value = row.value } })
        if (userStore.has('sysconfig:delete')) {
          btns.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
        }
      }
      return renderActions(btns)
    },
  },
])

async function load() {
  loading.value = true
  try {
    const res = await listSysConfigs(kw.value.trim() || undefined)
    rows.value = res.data.data
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, { key: '', value: '', type: 'string', description: '' })
  showModal.value = true
}

async function save() {
  saving.value = true
  try {
    await createSysConfig({ ...form })
    message.success(t('common.success'))
    showModal.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    saving.value = false
  }
}

async function saveEdit(row: SysConfig) {
  try {
    await updateSysConfig(row.key, { value: editValue.value })
    message.success(t('common.success'))
    editingKey.value = ''
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  }
}

function confirmDelete(row: SysConfig) {
  dialog.warning({
    title: t('common.tips'),
    content: t('sysconfig.deleteConfirm', { key: row.key }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteSysConfig(row.key)
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
.mono {
  font-family: var(--sx-font-mono);
}
</style>
