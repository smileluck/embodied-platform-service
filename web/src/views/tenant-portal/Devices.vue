<template>
  <SearchCard storage-key="tenant-devices" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('tenantPortal.devices.kwPlaceholder')" clearable style="width: 200px" @keyup.enter="search" />
    <n-select v-model:value="query.status" :options="statusOptions" clearable :placeholder="t('tenantPortal.devices.statusPlaceholder')" style="width: 140px" />
    <n-select v-model:value="query.online" :options="onlineOptions" clearable :placeholder="t('tenantPortal.devices.onlinePlaceholder')" style="width: 120px" />
  </SearchCard>

  <n-card>
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NCard, NDataTable, NTag, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { listTenantDevices } from '../../api/tenant'
import { usePagination } from '../../utils/pagination'
import type { TenantDevice } from '../../api/tenant'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)
const rows = ref<TenantDevice[]>([])
const query = reactive({
  kw: '',
  status: null as string | null,
  online: null as boolean | null,
  page: 1,
  page_size: 10,
})

const statusOptions = computed(() => [
  { label: 'active', value: 'active' },
  { label: 'inactive', value: 'inactive' },
  { label: 'disabled', value: 'disabled' },
  { label: 'retired', value: 'retired' },
])
const onlineOptions = computed(() => [
  { label: t('tenantPortal.devices.online'), value: true },
  { label: t('tenantPortal.devices.offline'), value: false },
])

const { pagination, setTotal, runSearch: search } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listTenantDevices({
      page: query.page,
      page_size: query.page_size,
      kw: query.kw || undefined,
      status: query.status ?? undefined,
      online: query.online ?? undefined,
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
  query.kw = ''
  query.status = null
  query.online = null
  query.page = 1
  load()
}

const columns = computed<DataTableColumns<TenantDevice>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('tenantPortal.devices.sn'), key: 'sn', width: 160 },
  { title: t('tenantPortal.devices.name'), key: 'name', width: 150, render: (row) => row.name || '—' },
  { title: t('tenantPortal.devices.model'), key: 'model_id', width: 90, render: (row) => `#${row.model_id}` },
  {
    title: t('tenantPortal.devices.status'), key: 'status', width: 110,
    render: (row) => h(NTag, {
      type: row.status === 'active' ? 'success' : row.status === 'disabled' ? 'error' : 'default',
      size: 'small', bordered: false,
    }, { default: () => row.status }),
  },
  {
    title: t('tenantPortal.devices.online'), key: 'online', width: 90,
    render: (row) => h(NTag, { type: row.online ? 'success' : 'default', size: 'small', bordered: false }, { default: () => (row.online ? t('tenantPortal.devices.online') : t('tenantPortal.devices.offline')) }),
  },
  { title: t('tenantPortal.devices.lastSeen'), key: 'last_seen_at', width: 170, render: (row) => row.last_seen_at || '—' },
  {
    title: t('common.operation'), key: 'actions', width: 90,
    render: (row) => h(NButton, { size: 'small', quaternary: true, type: 'primary', onClick: () => router.push(`/tenant/devices/${row.id}`) }, { default: () => t('common.detail') }),
  },
])

onMounted(load)
</script>
