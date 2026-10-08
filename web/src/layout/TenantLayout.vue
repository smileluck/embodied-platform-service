<template>
  <n-layout has-sider style="height: 100vh">
    <n-layout-sider class="sider" collapse-mode="width" :collapsed-width="64" :width="200" :collapsed="collapsed" show-trigger="bar">
      <div class="logo" :class="{ 'logo-collapsed': collapsed }">
        <div class="seal">T</div>
        <div class="logo-text">
          <span class="logo-name">SmileX</span>
          <span class="logo-sub mono">tenant portal</span>
        </div>
      </div>
      <n-menu
        :collapsed="collapsed" :collapsed-width="64" :collapsed-icon-size="20" :root-indent="16" :indent="20"
        :options="menuOptions" :value="activeKey" @update:value="onMenuSelect"
      />
      <div class="sider-foot mono" :class="{ 'foot-collapsed': collapsed }">v1.0<span class="foot-ext"> · tenant</span></div>
    </n-layout-sider>

    <n-layout class="main">
      <n-layout-header class="header">
        <div class="header-left">
          <span class="crumb-title">{{ currentTitle }}</span>
        </div>
        <div class="header-right">
          <!-- 租户切换器（多租户归属时可见；X-Tenant-ID 全链路口径=平台租户 ID） -->
          <n-select
            v-if="tenantOptions.length > 0" v-model:value="tenantSel" :options="tenantOptions"
            size="small" style="width: 180px" @update:value="onTenantSwitch"
          />
          <n-tooltip placement="bottom" :show-arrow="false" :delay="400">
            <template #trigger>
              <n-button quaternary circle :focusable="false" :aria-label="isDarkRef ? t('layout.toLight') : t('layout.toDark')" @click="toggleTheme">
                <template #icon>
                  <n-icon :component="isDarkRef ? SunnyOutline : MoonOutline" />
                </template>
              </n-button>
            </template>
            {{ isDarkRef ? t('layout.toLight') : t('layout.toDark') }}
          </n-tooltip>
          <n-dropdown :options="userDropdown" @select="onUserAction">
            <n-button quaternary size="small" class="user-chip">
              <span class="user-name">{{ store.user?.nickname || store.user?.username || '—' }}</span>
              <n-tag v-if="store.isAdmin" size="tiny" type="primary" :bordered="false">{{ t('tenantPortal.role.admin') }}</n-tag>
            </n-button>
          </n-dropdown>
        </div>
      </n-layout-header>
      <n-layout-content class="content">
        <router-view v-if="store.contextLoaded && store.tenantId" />
        <n-spin v-else class="boot-spin" />
      </n-layout-content>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NButton, NDropdown, NIcon, NLayout, NLayoutContent, NLayoutHeader, NLayoutSider,
  NMenu, NSelect, NSpin, NTag, NTooltip, type DropdownOption, type MenuOption,
} from 'naive-ui'
import { MoonOutline, SunnyOutline, PeopleOutline, HardwareChipOutline, PersonOutline } from '@vicons/ionicons5'
import { useTenantUserStore } from '../stores/tenantUser'
import { isDarkRef, toggleTheme } from '../stores/theme'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useTenantUserStore()
const collapsed = ref(false)

const tenantOptions = computed(() =>
  store.accessibleTenants.map((x) => ({ label: x.name, value: x.platform_id })),
)
const tenantSel = ref(store.tenantId)

// 菜单：静态定义按角色过滤（member 不见成员管理；无后端菜单面）
const menuOptions = computed<MenuOption[]>(() => {
  const opts: MenuOption[] = [
    { label: t('tenantPortal.menu.devices'), key: '/tenant-portal/devices', icon: () => h(NIcon, null, { default: () => h(HardwareChipOutline) }) },
  ]
  if (store.isAdmin) {
    opts.push({ label: t('tenantPortal.menu.members'), key: '/tenant-portal/members', icon: () => h(NIcon, null, { default: () => h(PeopleOutline) }) })
  }
  opts.push({ label: t('tenantPortal.menu.profile'), key: '/tenant-portal/profile', icon: () => h(NIcon, null, { default: () => h(PersonOutline) }) })
  return opts
})

const activeKey = computed(() => {
  // 详情页高亮所属菜单
  if (route.path.startsWith('/tenant-portal/devices')) return '/tenant-portal/devices'
  if (route.path.startsWith('/tenant-portal/members')) return '/tenant-portal/members'
  if (route.path.startsWith('/tenant-portal/profile')) return '/tenant-portal/profile'
  return ''
})

const currentTitle = computed(() => {
  const key = route.meta?.titleKey as string | undefined
  return key ? t(key) : ''
})

function onMenuSelect(key: string) {
  router.push(key)
}

// 切换租户：写 store 并整页重载（所有页面数据随 X-Tenant-ID 变化，重载最稳）
function onTenantSwitch(value: number) {
  store.switchTenant(value)
  window.location.href = '/tenant-portal/devices'
}

const userDropdown = computed<DropdownOption[]>(() => [
  { label: t('tenantPortal.menu.profile'), key: 'profile' },
  { type: 'divider', key: 'd1' },
  { label: t('tenantPortal.logout'), key: 'logout' },
])

async function onUserAction(key: string | number) {
  if (key === 'profile') {
    router.push('/tenant-portal/profile')
  } else if (key === 'logout') {
    await store.logout()
    window.location.href = '/tenant-portal/login'
  }
}

// 外部（登录流程）更新 store.tenantId 后同步下拉选中值
watch(() => store.tenantId, (v) => { tenantSel.value = v })
</script>

<style scoped>
.sider {
  display: flex;
  flex-direction: column;
}
.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px;
  overflow: hidden;
  white-space: nowrap;
}
.logo-collapsed {
  padding: 16px 12px;
}
.seal {
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 9px;
  font-family: var(--sx-font-mono);
  font-weight: 700;
  font-size: 17px;
  color: #fff;
  background: var(--sx-accent);
}
.logo-text {
  display: flex;
  flex-direction: column;
}
.logo-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--sx-ink);
}
.logo-sub {
  font-family: var(--sx-font-mono);
  font-size: 10px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--sx-muted);
}
.sider-foot {
  padding: 14px 16px;
  font-size: 11px;
  color: var(--sx-muted);
}
.foot-collapsed .foot-ext {
  display: none;
}
.main {
  background: var(--sx-bg);
}
.header {
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  border-bottom: 1px solid var(--sx-line);
  background: var(--sx-panel);
}
.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.crumb-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--sx-ink);
}
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.user-chip {
  gap: 6px;
}
.user-name {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.content {
  padding: 16px;
  height: calc(100vh - 52px);
  overflow: auto;
}
.boot-spin {
  margin: 120px auto;
  display: block;
}
</style>
