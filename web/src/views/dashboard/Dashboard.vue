<template>
  <n-grid :cols="4" :x-gap="16">
    <n-gi v-for="card in cards" :key="card.label">
      <n-card>
        <n-statistic :label="card.label" :value="card.value" />
      </n-card>
    </n-gi>
  </n-grid>
  <n-card :title="t('dashboard.welcome')" style="margin-top: 16px">
    <p>{{ t('dashboard.greeting', { name: userStore.user?.nickname || userStore.user?.username }) }}</p>
    <p style="color: #999; font-size: 13px">
      {{ t('dashboard.techLine') }}
    </p>
  </n-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NGrid, NGi, NCard, NStatistic } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useUserStore } from '../../stores/user'
import { listUsers, listRoles, listPermissions } from '../../api'

const { t } = useI18n()
const userStore = useUserStore()
const totals = ref({ users: 0, roles: 0, perms: 0 })
const cards = computed(() => [
  { label: t('dashboard.userCount'), value: totals.value.users },
  { label: t('dashboard.roleCount'), value: totals.value.roles },
  { label: t('dashboard.permCount'), value: totals.value.perms },
  { label: t('dashboard.myPermCodes'), value: userStore.permissions.length },
])

onMounted(async () => {
  const [u, r, p] = await Promise.all([
    listUsers({ page: 1, page_size: 1 }),
    listRoles({ page: 1, page_size: 1 }),
    listPermissions({ page: 1, page_size: 1 }),
  ])
  totals.value.users = u.data.data.page.total
  totals.value.roles = r.data.data.page.total
  totals.value.perms = p.data.data.page.total
})
</script>
