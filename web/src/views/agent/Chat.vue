<template>
  <!-- 智能体底座 · 聊天测试：左右 4:6 固定分栏（同 Providers 页范式），左选 Agent 右对话 -->
  <div class="chat-page">
    <div class="cols">
      <!-- 左栏：Agent 列表（客户端过滤，禁用的不可选） -->
      <div class="col col-left">
        <n-card size="small" class="agents-card">
          <template #header>
            <span class="card-title">{{ t('agent.chat.agentList') }}</span>
          </template>
          <div class="agents-toolbar">
            <n-input v-model:value="kw" size="small" :placeholder="t('agent.chat.searchPlaceholder')" clearable />
          </div>
          <div v-if="loading" class="agents-loading"><n-spin size="small" /></div>
          <n-empty v-else-if="!filtered.length" size="small" :description="t('agent.chat.emptyAgent')" style="padding: 24px 0" />
          <div v-else class="agents-list">
            <div
              v-for="a in filtered" :key="a.id"
              class="agent-item" :class="{ active: selected?.id === a.id, off: a.status !== 1 }"
              @click="select(a)"
            >
              <div class="agent-row">
                <span class="agent-name">{{ a.name }}</span>
                <n-tag size="small" :type="a.status === 1 ? 'success' : 'default'" :bordered="false">
                  {{ a.status === 1 ? t('common.enabled') : t('common.disabled') }}
                </n-tag>
              </div>
              <div class="agent-meta mono">{{ a.code }} · {{ modelLabel(a) }}</div>
            </div>
          </div>
        </n-card>
      </div>

      <!-- 右栏：对话区（切换 Agent 重挂载，会话重置） -->
      <div class="col col-right">
        <n-card size="small" class="chat-card">
          <template #header>
            <span class="card-title">{{ selected ? `${t('agent.chat.title')} · ${selected.name}` : t('agent.chat.title') }}</span>
          </template>
          <ChatPanel
            v-if="selected" :key="selected.id"
            :agent-id="selected.id" :model-label="modelLabel(selected)" :rows="4"
          />
          <n-empty v-else size="small" :description="t('agent.chat.selectHint')" style="padding: 60px 0" />
        </n-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { NCard, NEmpty, NInput, NSpin, NTag } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import ChatPanel from './ChatPanel.vue'
import { listAgentModels, listAgentProviders, listAgents } from '../../api'
import type { AgentInfo, AgentModel, AgentProvider } from '../../api/types'

const { t } = useI18n()
const route = useRoute()

const loading = ref(false)
const agents = ref<AgentInfo[]>([])
const providers = ref<AgentProvider[]>([])
const models = ref<AgentModel[]>([])
const kw = ref('')
const selected = ref<AgentInfo | null>(null)

const providerName = computed(() => new Map(providers.value.map((p) => [p.id, p.name])))
const modelById = computed(() => new Map(models.value.map((m) => [m.id, m])))

// 名称/编码包含匹配（客户端过滤，列表一次拉全）
const filtered = computed(() => {
  const k = kw.value.trim().toLowerCase()
  if (!k) return agents.value
  return agents.value.filter(
    (a) => a.name.toLowerCase().includes(k) || a.code.toLowerCase().includes(k),
  )
})

function modelLabel(a: AgentInfo): string {
  const m = modelById.value.get(a.model_id)
  if (!m) return '—'
  return `${providerName.value.get(m.provider_id) ?? ''} · ${m.display_name || m.name}`
}

// 禁用的 Agent 不可选（对话会被后端拒绝），保持可见以便辨识
function select(a: AgentInfo) {
  if (a.status !== 1) return
  selected.value = a
}

onMounted(async () => {
  loading.value = true
  try {
    const [aRes, pRes, mRes] = await Promise.all([
      listAgents({ page: 1, page_size: 100 }),
      listAgentProviders({ page: 1, page_size: 100 }),
      listAgentModels({ page: 1, page_size: 100 }),
    ])
    agents.value = aRes.data.data.list
    providers.value = pRes.data.data.list
    models.value = mRes.data.data.list
    // 支持 /agent/chat?agent=<id> 预选（深链/页间跳转）
    const qid = Number(route.query.agent) || 0
    selected.value =
      agents.value.find((a) => a.id === qid && a.status === 1) ??
      agents.value.find((a) => a.status === 1) ??
      null
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
/* 撑满一屏：100vh - 顶栏 64px - 内容区上下 padding（与 Providers/ServerMonitor 同范式） */
.chat-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 64px - 16px);
  min-height: 0;
}
.cols {
  flex: 1;
  display: flex;
  gap: 12px;
  min-height: 0;
}
.col {
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
}
.col-left {
  flex: 4 1 0;
  overflow: auto;
}
.col-right {
  flex: 6 1 0;
  overflow: hidden; /* 右栏滚动交给对话面板内部 */
}

.card-title {
  font-weight: 600;
}

/* 左栏 Agent 卡片：卡内列表滚动 */
.agents-card {
  display: flex;
  flex-direction: column;
}
.agents-card :deep(.n-card__content) {
  flex: 1;
  overflow: auto;
  min-height: 0;
}
.agents-toolbar {
  margin-bottom: 8px;
}
.agents-loading {
  padding: 24px 0;
  text-align: center;
}
.agents-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.agent-item {
  padding: 8px 10px;
  border: 1px solid var(--sx-line);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}
.agent-item:hover {
  border-color: var(--sx-accent);
}
.agent-item.active {
  border-color: var(--sx-accent);
  background-color: rgba(63, 117, 171, 0.08);
}
.agent-item.off {
  cursor: not-allowed;
  opacity: 0.55;
}
.agent-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.agent-name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.agent-meta {
  margin-top: 2px;
  font-size: 12px;
  color: var(--sx-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 右栏对话卡：撑满高度，滚动在 ChatPanel 内部 */
.chat-card {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.chat-card :deep(.n-card__content) {
  flex: 1;
  min-height: 0;
}

.mono {
  font-family: var(--sx-font-mono);
}
</style>
