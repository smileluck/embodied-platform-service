<!-- 搜索栏独立卡片：筛选项 + 右下角「重置 / 搜索」按钮，卡头可点击折叠。
     storageKey 提供时折叠状态按页持久化（仿侧栏 sider_collapsed 模式）。 -->
<template>
  <n-card size="small" class="search-card">
    <template #header>
      <div class="search-toggle" role="button" :aria-expanded="!collapsed" @click="toggle">
        <span class="search-toggle-title">{{ t('common.search') }}</span>
        <n-icon :class="['chevron', { collapsed }]">
          <ChevronDownOutline />
        </n-icon>
      </div>
    </template>
    <n-collapse-transition :show="!collapsed">
      <div class="search-fields">
        <slot />
      </div>
      <div class="search-actions">
        <n-button quaternary @click="emit('reset')">{{ t('common.reset') }}</n-button>
        <n-button type="primary" @click="emit('search')">{{ t('common.search') }}</n-button>
      </div>
    </n-collapse-transition>
  </n-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NCard, NCollapseTransition, NIcon } from 'naive-ui'
import { ChevronDownOutline } from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{ storageKey?: string }>()
const emit = defineEmits<{ (e: 'search'): void; (e: 'reset'): void }>()

// 折叠状态持久化：默认展开，仅记录过折叠的页面保持折叠
const collapsed = ref(props.storageKey ? localStorage.getItem(`search_collapsed:${props.storageKey}`) === '1' : false)

function toggle() {
  collapsed.value = !collapsed.value
  if (props.storageKey) {
    localStorage.setItem(`search_collapsed:${props.storageKey}`, collapsed.value ? '1' : '0')
  }
}
</script>

<style scoped>
/* 与下方表格卡片的间距 */
.search-card {
  margin-bottom: 8px;
}
/* 卡头整行可点击折叠 */
.search-toggle {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}
.search-toggle-title {
  font-size: 13px;
  color: var(--sx-muted);
}
/* 箭头随状态旋转：收起朝下（可展开）、展开朝上（可收起） */
.chevron {
  color: var(--sx-muted);
  transition: transform 0.2s ease;
  transform: rotate(180deg);
}
.chevron.collapsed {
  transform: rotate(0deg);
}
/* 筛选项：增多或窄视口时自动换行 */
.search-fields {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
/* 操作按钮固定右下角，上方发丝线分区（沿用列表页原搜索栏的分区语言） */
.search-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--sx-line);
}
</style>
