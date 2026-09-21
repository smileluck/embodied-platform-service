<template>
  <SearchCard storage-key="ipBlacklist" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.ip" :placeholder="t('blacklist.ipPlaceholder')" clearable style="width: 180px" @keyup.enter="load" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-header">
        <span>{{ t('blacklist.title') }}</span>
        <n-button type="primary" v-permission="['blacklist:create']" @click="showModal = true">{{ t('common.add') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <n-modal v-model:show="showModal" :title="t('blacklist.addTitle')" preset="card" style="width: 420px" :bordered="false" @after-leave="resetForm">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('blacklist.ip')" path="ip">
        <n-input v-model:value="form.ip" :placeholder="t('blacklist.ipPlaceholder')" clearable />
      </n-form-item>
      <n-form-item :label="t('blacklist.reason')" path="reason">
        <n-input v-model:value="form.reason" :placeholder="t('blacklist.reasonPlaceholder')" clearable type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
      </n-form-item>
      <n-form-item :label="t('blacklist.expireAt')" path="expire_at">
        <n-date-picker v-model:value="form.expire_at" type="datetime" clearable style="width: 100%" :placeholder="t('blacklist.expireAtPlaceholder')" />
      </n-form-item>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="submitting" @click="handleSubmit">{{ t('common.save') }}</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NButton, NCard, NDataTable, NDatePicker, NForm, NFormItem, NInput, NModal, NSpace, NTag, useDialog, useMessage, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { createBlacklist, deleteBlacklist, listBlacklist } from '../../api'
import { usePagination } from '../../utils/pagination'
import type { BlacklistItem } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const rows = ref<BlacklistItem[]>([])
const query = reactive({ ip: '', page: 1, page_size: 10 })

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listBlacklist({
      page: query.page,
      page_size: query.page_size,
      ip: query.ip || undefined,
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
  query.ip = ''
  query.page = 1
  load()
}

const showModal = ref(false)
const submitting = ref(false)
const formRef = ref<FormInst>()
const form = reactive<{ ip: string; reason: string; expire_at: number | null }>({ ip: '', reason: '', expire_at: null })

const rules: FormRules = {
  ip: [{ required: true, message: () => t('blacklist.ipRequired'), trigger: 'blur' }],
}

function resetForm() {
  form.ip = ''
  form.reason = ''
  form.expire_at = null
}

async function handleSubmit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  submitting.value = true
  try {
    await createBlacklist({
      ip: form.ip.trim(),
      reason: form.reason,
      expire_at: form.expire_at ? Math.floor(form.expire_at / 1000) : null,
    })
    message.success(t('common.saveSuccess'))
    showModal.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    submitting.value = false
  }
}

function confirmDelete(row: BlacklistItem) {
  dialog.warning({
    title: t('common.tips'),
    content: t('blacklist.deleteConfirm', { ip: row.ip }),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteBlacklist(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

const columns = computed<DataTableColumns<BlacklistItem>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('blacklist.ip'), key: 'ip', width: 160 },
  { title: t('blacklist.reason'), key: 'reason', width: 300, ellipsis: { tooltip: true }, render: (row) => row.reason || '—' },
  {
    title: t('blacklist.source'), key: 'source', width: 90,
    render: (row) => h(NTag, { size: 'small', type: row.source === 'auto' ? 'warning' : 'default', bordered: false },
      { default: () => row.source === 'auto' ? t('blacklist.sourceAuto') : t('blacklist.sourceManual') }),
  },
  {
    // 封禁时间 + 标识合并列：永久封禁=「永久」tag；限期封禁=「限期」tag + 到期时间
    title: t('blacklist.bannedAt'), key: 'created_at', width: 300,
    render: (row) => h('div', { class: 'banned-cell' }, [
      h('span', { class: 'mono' }, row.created_at),
      h(NTag, {
        size: 'small', bordered: false,
        type: row.expire_at ? 'warning' : 'default',
      }, { default: () => (row.expire_at ? `${t('blacklist.until')} ${row.expire_at}` : t('blacklist.permanent')) }),
    ]),
  },
  { title: t('blacklist.creator'), key: 'creator_name', width: 120, render: (row) => row.creator_name || '—' },
  {
    title: t('common.operation'), key: 'actions', fixed: 'right', width: 100,
    render: (row) => h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(row) }, { default: () => t('blacklist.unblock') }),
  },
])

onMounted(load)
</script>

<style scoped>
.page-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
</style>

<style>
/* .banned-cell 由表格列渲染函数 h() 创建，拿不到本组件 scoped 的 data-v 属性，需全局规则 */
.banned-cell {
  display: flex;
  flex-wrap: wrap; /* 列宽不足时限期 tag 换到时间下一行显示 */
  align-items: center;
  gap: 4px 8px;
  font-size: 12px;
}
</style>
