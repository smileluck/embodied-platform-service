<template>
  <!-- 智能体底座 · 聊天测试：左右 4:6 固定分栏，左栏上选 Agent 下选会话，右栏对话 -->
  <div class="chat-page">
    <div class="cols">
      <!-- 左栏：Agent 列表 + 会话列表（客户端过滤，禁用的不可选） -->
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

        <!-- 会话列表：选中 Agent 后展示（消息持久化） -->
        <n-card v-if="selected" size="small" class="convs-card">
          <template #header>
            <span class="card-title">{{ t('agent.chat.conversations') }}</span>
          </template>
          <template #header-extra>
            <n-button size="tiny" type="primary" ghost :disabled="!canCreateConv" @click="newConversation">
              {{ t('agent.chat.newConversation') }}
            </n-button>
          </template>
          <div v-if="convsLoading" class="agents-loading"><n-spin size="small" /></div>
          <n-empty v-else-if="!conversations.length" size="small" :description="t('agent.chat.emptyConversations')" style="padding: 24px 0" />
          <div v-else class="convs-list">
            <div
              v-for="cv in conversations" :key="cv.id"
              class="conv-item" :class="{ active: selectedConv?.id === cv.id }"
              @click="selectedConv = cv"
            >
              <div class="conv-title">{{ cv.title }}</div>
              <div class="conv-meta mono">{{ fmtTime(cv.last_msg_at) }}</div>
              <div class="conv-ops">
                <n-button size="tiny" quaternary @click.stop="openRename(cv)">{{ t('agent.chat.rename') }}</n-button>
                <n-button size="tiny" quaternary type="error" @click.stop="confirmDelete(cv)">✕</n-button>
              </div>
            </div>
          </div>
        </n-card>
      </div>

      <!-- 右栏：对话区（切换会话重挂载加载历史；未选会话为无状态调试） -->
      <div class="col col-right">
        <n-card size="small" class="chat-card">
          <template #header>
            <span class="card-title">{{ selected ? `${t('agent.chat.title')} · ${selected.name}` : t('agent.chat.title') }}</span>
          </template>
          <ChatPanel
            v-if="selected" :key="`${selected.id}-${selectedConv?.id ?? 0}`"
            :agent-id="selected.id" :conversation-id="selectedConv?.id ?? 0"
            :model-label="modelLabel(selected)" :rows="4"
          />
          <n-empty v-else size="small" :description="t('agent.chat.selectHint')" style="padding: 60px 0" />
        </n-card>
      </div>
    </div>

    <!-- 重命名会话 -->
    <n-modal v-model:show="renameShow" preset="dialog" :title="t('agent.chat.renameTitle')" :positive-text="t('common.confirm')" :negative-text="t('common.cancel')" @positive="submitRename">
      <n-form>
        <n-form-item :label="t('agent.chat.newTitle')" :feedback="renameFeedback" :validation-status="renameFeedback ? 'error' : undefined">
          <n-input v-model:value="renameTitle" :maxlength="20" show-count @keydown.enter.prevent="submitRename" />
        </n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCard, NEmpty, NForm, NFormItem, NInput, NModal, NSpin, NTag, useDialog, useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import ChatPanel from './ChatPanel.vue'
import {
  createAgentConversation, deleteAgentConversation, listAgentConversations,
  listAgentModels, listAgentProviders, listAgents, renameAgentConversation,
} from '../../api'
import type { AgentConversation, AgentInfo, AgentModel, AgentProvider } from '../../api/types'
import { useUserStore } from '../../stores/user'

const { t } = useI18n()
const route = useRoute()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const agents = ref<AgentInfo[]>([])
const providers = ref<AgentProvider[]>([])
const models = ref<AgentModel[]>([])
const kw = ref('')
const selected = ref<AgentInfo | null>(null)

// 会话列表（当前 Agent 的本人会话）
const conversations = ref<AgentConversation[]>([])
const selectedConv = ref<AgentConversation | null>(null)
const convsLoading = ref(false)

// 重命名弹窗
const renameShow = ref(false)
const renameTitle = ref('')
const renameTarget = ref<AgentConversation | null>(null)

const canCreateConv = computed(() => userStore.has('agent:conversation:create'))
const renameFeedback = computed(() => {
  if (!renameTitle.value.trim()) return t('agent.chat.titleRequired')
  return ''
})

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

function fmtTime(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  return sameDay ? hm : `${d.getMonth() + 1}-${String(d.getDate()).padStart(2, '0')} ${hm}`
}

// 选中 Agent：加载其会话并自动选中最近一个（无会话则无状态模式，点新建后开始持久对话）
function select(a: AgentInfo) {
  if (a.status !== 1 || selected.value?.id === a.id) return
  selected.value = a
  selectedConv.value = null
  loadConversations(a.id)
}

async function loadConversations(agentId: number) {
  convsLoading.value = true
  try {
    const res = await listAgentConversations({ page: 1, page_size: 50, agent_id: agentId })
    conversations.value = res.data.data.list
    selectedConv.value = conversations.value[0] ?? null
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    convsLoading.value = false
  }
}

async function newConversation() {
  if (!selected.value) return
  try {
    const res = await createAgentConversation(selected.value.id)
    conversations.value.unshift(res.data.data)
    selectedConv.value = res.data.data
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.saveFailed'))
  }
}

function openRename(cv: AgentConversation) {
  renameTarget.value = cv
  renameTitle.value = cv.title
  renameShow.value = true
}

async function submitRename() {
  const cv = renameTarget.value
  if (!cv || renameFeedback.value) return
  try {
    const res = await renameAgentConversation(cv.id, renameTitle.value.trim())
    const i = conversations.value.findIndex((c) => c.id === cv.id)
    if (i >= 0) conversations.value[i] = res.data.data
    if (selectedConv.value?.id === cv.id) selectedConv.value = res.data.data
    renameShow.value = false
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.saveFailed'))
  }
}

function confirmDelete(cv: AgentConversation) {
  dialog.warning({
    title: t('common.tips'),
    content: t('agent.chat.deleteConfirm', { title: cv.title }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteAgentConversation(cv.id)
        conversations.value = conversations.value.filter((c) => c.id !== cv.id)
        if (selectedConv.value?.id === cv.id) selectedConv.value = conversations.value[0] ?? null
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('agent.deleteFailed'))
      }
    },
  })
}

onMounted(async () => {
  loading.value = true
  try {
    const [aRes, pRes, mRes] = await Promise.all([
      listAgents({ page: 1, page_size: 0 }),
      listAgentProviders({ page: 1, page_size: 0 }),
      listAgentModels({ page: 1, page_size: 0 }),
    ])
    agents.value = aRes.data.data.list
    providers.value = pRes.data.data.list
    models.value = mRes.data.data.list
    // 支持 /agent/chat?agent=<id> 预选（深链/页间跳转）
    const qid = Number(route.query.agent) || 0
    const pick = agents.value.find((a) => a.id === qid && a.status === 1) ?? agents.value.find((a) => a.status === 1) ?? null
    selected.value = pick
    if (pick) loadConversations(pick.id)
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
  gap: 12px;
  overflow: auto;
}
.col-right {
  flex: 6 1 0;
  overflow: hidden; /* 右栏滚动交给对话面板内部 */
}

.card-title {
  font-weight: 600;
}

/* 左栏 Agent 卡：占上部，卡内列表滚动 */
.agents-card {
  flex: 1 1 55%;
  display: flex;
  flex-direction: column;
  min-height: 220px;
}
.agents-card :deep(.n-card__content) {
  flex: 1;
  overflow: auto;
  min-height: 0;
}
/* 左栏会话卡：占下部 */
.convs-card {
  flex: 1 1 45%;
  display: flex;
  flex-direction: column;
  min-height: 180px;
}
.convs-card :deep(.n-card__content) {
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

/* 会话列表项：标题 + 时间 + hover 操作 */
.convs-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.conv-item {
  position: relative;
  padding: 7px 10px;
  border: 1px solid var(--sx-line);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}
.conv-item:hover {
  border-color: var(--sx-accent);
}
.conv-item.active {
  border-color: var(--sx-accent);
  background-color: rgba(63, 117, 171, 0.08);
}
.conv-title {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding-right: 4px;
}
.conv-meta {
  margin-top: 1px;
  font-size: 11px;
  color: var(--sx-muted);
}
.conv-ops {
  position: absolute;
  top: 50%;
  right: 6px;
  transform: translateY(-50%);
  display: none;
  gap: 2px;
}
.conv-item:hover .conv-ops {
  display: flex;
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
