<template>
  <!-- Agent 聊天面板：SSE 流式对话，供 调试抽屉 与 聊天测试页 复用。
       传入 conversation 时加载历史并持久化消息；不传则为无状态调试（不落库） -->
  <div class="chat-panel">
    <div class="chat-head">
      <span class="chat-model mono">{{ currentModel || '—' }}</span>
      <span class="chat-sub">{{ subtitle }}</span>
    </div>
    <div ref="listRef" class="chat-list" @click="onListClick">
      <div v-if="loadingHistory" class="chat-empty"><n-spin size="small" /></div>
      <div v-else-if="!messages.length" class="chat-empty">{{ t('agent.playground.empty') }}</div>
      <div v-for="(m, i) in messages" :key="i" class="chat-msg" :class="m.role">
        <div class="chat-bubble">
          <n-button
            v-if="m.content" class="bubble-copy" size="tiny" quaternary
            :title="t('agent.playground.copy')" @click="copyText(m.content)"
          >⧉</n-button>
          <div v-for="(tc, j) in m.toolCalls || []" :key="'t' + j" class="tool-card">
            <button type="button" class="tool-head" @click="m.toolOpen = !m.toolOpen">
              <span class="tool-name mono">🔧 {{ tc.name }}</span>
              <span class="tool-flag" :class="{ err: !!tc.error }">{{ tc.error ? t('agent.playground.toolFailed') : t('agent.playground.toolDone') }}</span>
              <span class="tool-arrow">{{ m.toolOpen ? '▾' : '▸' }}</span>
            </button>
            <div v-show="m.toolOpen" class="tool-body mono">
              <div class="tool-sec">{{ t('agent.playground.toolArgs') }}</div>
              <div class="tool-args">{{ tc.arguments || '{}' }}</div>
              <div class="tool-sec">{{ t('agent.playground.toolResult') }}</div>
              <div class="tool-result" :class="{ err: !!tc.error }">{{ tc.error || tc.result }}</div>
            </div>
          </div>
          <div v-if="m.role === 'assistant'" class="chat-text md" v-html="renderMd(m)"></div>
          <div v-else class="chat-text">{{ m.content }}</div>
          <div v-if="m.error" class="chat-error">{{ m.error }}</div>
          <div v-else-if="m.role === 'assistant' && !m.streaming && m.usage" class="chat-usage mono">
            {{ t('agent.playground.tokenUsage') }} {{ m.usage.total_tokens }}
          </div>
        </div>
      </div>
    </div>
    <div class="chat-input">
      <n-input
        v-model:value="input" type="textarea" :rows="rows" :maxlength="4000"
        :placeholder="canChat ? t('agent.playground.inputPlaceholder') : t('agent.playground.noPermission')"
        :disabled="sending || !canChat"
        @keydown.enter.exact.prevent="send"
      />
      <div class="chat-actions">
        <n-button size="small" quaternary :disabled="!messages.length || sending" @click="clear">{{ t('agent.playground.clear') }}</n-button>
        <n-button v-if="sending" size="small" type="error" ghost @click="stop">{{ t('agent.playground.stop') }}</n-button>
        <n-button v-else size="small" type="primary" :disabled="!canChat || !input.trim()" @click="send">{{ t('agent.playground.send') }}</n-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { NButton, NInput, NSpin, useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { listAgentConversationMessages } from '../../api'
import { useUserStore } from '../../stores/user'
import { renderMarkdown } from '../../utils/markdown'

interface ToolCallResult {
  id: string
  name: string
  arguments: string
  result: string
  error?: string
}

interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
  error?: string
  usage?: { total_tokens: number }
  toolCalls?: ToolCallResult[] // 工具调用过程（function calling）
  toolOpen?: boolean // 过程卡片展开态
}

const props = withDefaults(defineProps<{
  agentId: number
  conversationId?: number // 会话 ID：传入则加载历史 + 消息落库；不传为无状态调试
  modelLabel?: string // 初始模型标签（后续以 SSE meta 帧为准）
  rows?: number // 输入框行数（页面场景更高）
}>(), { conversationId: 0, modelLabel: '', rows: 3 })

const { t, locale } = useI18n()
const userStore = useUserStore()
const message = useMessage()

const canChat = computed(() => userStore.has('agent:chat'))
const subtitle = computed(() =>
  props.conversationId ? t('agent.playground.subtitlePersist') : t('agent.playground.subtitle'))

const messages = ref<ChatMsg[]>([])
const input = ref('')
const sending = ref(false)
const loadingHistory = ref(false)
const abortCtrl = ref<AbortController | null>(null)
const listRef = ref<HTMLElement | null>(null)
const currentModel = ref(props.modelLabel)

// 新消息/流式增量后滚动到底部
watch(
  () => messages.value.map((m) => m.content.length).join(','),
  () => nextTick(() => {
    const el = listRef.value
    if (el) el.scrollTop = el.scrollHeight
  }),
)

onMounted(loadHistory)

// 加载会话历史消息（时间正序；无状态模式跳过）
async function loadHistory() {
  if (!props.conversationId) return
  loadingHistory.value = true
  try {
    const res = await listAgentConversationMessages(props.conversationId, { page: 1, page_size: 100 })
    messages.value = res.data.data.list.map((m) => ({
      role: m.role,
      content: m.content,
      usage: m.role === 'assistant' && m.total_tokens ? { total_tokens: m.total_tokens } : undefined,
    }))
    nextTick(() => {
      const el = listRef.value
      if (el) el.scrollTop = el.scrollHeight
    })
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.playground.loadHistoryFailed'))
  } finally {
    loadingHistory.value = false
  }
}

function clear() {
  messages.value = []
}

// assistant 消息 markdown 渲染（流式期间每个增量帧重渲染当前气泡）
function renderMd(m: ChatMsg): string {
  if (!m.content && m.streaming) return ''
  return renderMarkdown(m.content, { copyLabel: t('agent.playground.copy') })
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    message.success(t('agent.playground.copied'))
  } catch {
    message.error(t('agent.playground.copyFailed'))
  }
}

// 代码块复制按钮的容器级事件委托
function onListClick(e: MouseEvent) {
  const btn = (e.target as HTMLElement)?.closest?.('.md-copy-btn') as HTMLElement | null
  if (!btn) return
  const code = btn.closest('.md-code')?.querySelector('code')?.textContent ?? ''
  copyText(code)
}

function stop() {
  abortCtrl.value?.abort()
}

async function send() {
  const text = input.value.trim()
  if (!text || sending.value) return
  input.value = ''
  messages.value.push({ role: 'user', content: text })
  // 历史消息（system prompt 由后端按 Agent 配置注入）：排除出错的与空回复的 assistant
  const history = messages.value
    .filter((m) => !m.error && (m.role === 'user' || m.content))
    .map((m) => ({ role: m.role, content: m.content }))
  const assistant: ChatMsg = { role: 'assistant', content: '', streaming: true }
  messages.value.push(assistant)
  sending.value = true
  const ctrl = new AbortController()
  abortCtrl.value = ctrl
  try {
    const resp = await fetch(`/api/v1/agents/${props.agentId}/chat`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.accessToken}`,
        'Accept-Language': locale.value,
      },
      body: JSON.stringify({ messages: history, conversation_id: props.conversationId || undefined }),
      signal: ctrl.signal,
    })
    if (!resp.ok || !resp.body) {
      // 非 200：JSON 错误信封（RBAC 拒绝 / 参数错误 / 上游错误等）
      let msg = t('agent.playground.error')
      try {
        const j = await resp.json()
        if (j?.msg) msg = j.msg
      } catch { /* 忽略解析失败 */ }
      throw new Error(msg)
    }
    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      let idx: number
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const frame = buf.slice(0, idx)
        buf = buf.slice(idx + 2)
        handleSSEFrame(frame, assistant)
      }
    }
  } catch (e: any) {
    if (e?.name === 'AbortError') {
      assistant.content += `\n${t('agent.playground.stopped')}`
    } else {
      assistant.error = e?.message || t('agent.playground.error')
    }
  } finally {
    assistant.streaming = false
    sending.value = false
    abortCtrl.value = null
  }
}

// 解析一帧 SSE：event: xxx\ndata: {...}
function handleSSEFrame(frame: string, assistant: ChatMsg) {
  let event = 'message'
  const dataLines: string[] = []
  for (const line of frame.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (!dataLines.length) return
  let data: any
  try {
    data = JSON.parse(dataLines.join('\n'))
  } catch {
    return
  }
  if (event === 'meta') {
    if (data?.model) currentModel.value = String(data.model)
  } else if (event === 'delta') {
    if (data?.delta) assistant.content += data.delta
    if (data?.usage) assistant.usage = data.usage
  } else if (event === 'tool') {
    // 工具调用执行结果（function calling 过程）
    const calls: ToolCallResult[] = data?.tool_calls ?? []
    assistant.toolCalls = [...(assistant.toolCalls ?? []), ...calls]
    assistant.toolOpen = true
  } else if (event === 'error') {
    assistant.error = data?.message || t('agent.playground.error')
  }
}
</script>

<style scoped>
.chat-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.chat-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 8px;
}
.chat-model {
  font-size: 12px;
  color: var(--sx-muted);
}
.chat-sub {
  font-size: 12px;
  color: var(--sx-muted);
  opacity: 0.8;
}
.chat-list {
  flex: 1;
  overflow: auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px;
}
.chat-empty {
  color: var(--sx-muted);
  font-size: 13px;
  text-align: center;
  padding: 40px 0;
}
.chat-msg {
  display: flex;
}
.chat-msg.user {
  justify-content: flex-end;
}
.chat-bubble {
  max-width: 86%;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: 14px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}
.chat-msg.user .chat-bubble {
  background: rgba(63, 117, 171, 0.12);
}
.chat-msg.assistant .chat-bubble {
  background: var(--sx-surface);
  border: 1px solid var(--sx-line);
}
.chat-error {
  margin-top: 4px;
  font-size: 12px;
  color: var(--sx-danger);
}
.chat-usage {
  margin-top: 4px;
  font-size: 11px;
  color: var(--sx-muted);
}
.chat-input {
  border-top: 1px solid var(--sx-line);
  padding-top: 12px;
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.chat-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.mono {
  font-family: var(--sx-font-mono);
}

/* 工具调用过程卡片（可折叠） */
.tool-card {
  margin: 4px 0;
  border: 1px dashed var(--sx-line);
  border-radius: 8px;
  overflow: hidden;
  font-size: 12px;
}
.tool-head {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px;
  border: none;
  background: rgba(63, 117, 171, 0.06);
  cursor: pointer;
  color: inherit;
}
.tool-name {
  font-size: 12px;
  color: var(--sx-muted);
}
.tool-flag {
  font-size: 11px;
  color: #4c9a6d;
}
.tool-flag.err {
  color: var(--sx-danger);
}
.tool-arrow {
  margin-left: auto;
  color: var(--sx-muted);
  font-size: 11px;
}
.tool-body {
  padding: 8px 10px;
  border-top: 1px dashed var(--sx-line);
  max-height: 220px;
  overflow: auto;
}
.tool-sec {
  font-size: 11px;
  color: var(--sx-muted);
  margin: 2px 0;
}
.tool-args, .tool-result {
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 11.5px;
  line-height: 1.5;
}
.tool-result.err {
  color: var(--sx-danger);
}

/* 消息级复制按钮：hover 气泡时浮现 */
.bubble-copy {
  position: absolute;
  top: 4px;
  right: 4px;
  opacity: 0;
  transition: opacity 0.15s;
}
.chat-bubble {
  position: relative;
}
.chat-bubble:hover .bubble-copy {
  opacity: 1;
}

/* markdown 排版（v-html 内容需 :deep） */
.chat-text.md :deep(p) {
  margin: 0 0 6px;
}
.chat-text.md :deep(p:last-child) {
  margin-bottom: 0;
}
.chat-text.md :deep(ul), .chat-text.md :deep(ol) {
  margin: 4px 0;
  padding-left: 20px;
}
.chat-text.md :deep(li) {
  margin: 2px 0;
}
.chat-text.md :deep(h1), .chat-text.md :deep(h2), .chat-text.md :deep(h3),
.chat-text.md :deep(h4), .chat-text.md :deep(h5), .chat-text.md :deep(h6) {
  margin: 10px 0 6px;
  font-size: 14px;
  font-weight: 600;
}
.chat-text.md :deep(a) {
  color: var(--sx-accent);
  text-decoration: none;
}
.chat-text.md :deep(a:hover) {
  text-decoration: underline;
}
.chat-text.md :deep(blockquote) {
  margin: 6px 0;
  padding: 2px 10px;
  border-left: 3px solid var(--sx-line);
  color: var(--sx-muted);
}
.chat-text.md :deep(code) {
  font-family: var(--sx-font-mono);
  font-size: 12.5px;
}
.chat-text.md :deep(p code), .chat-text.md :deep(li code) {
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(63, 117, 171, 0.1);
}
.chat-text.md :deep(table) {
  border-collapse: collapse;
  margin: 6px 0;
  font-size: 13px;
}
.chat-text.md :deep(th), .chat-text.md :deep(td) {
  border: 1px solid var(--sx-line);
  padding: 4px 8px;
}
.chat-text.md :deep(hr) {
  border: none;
  border-top: 1px solid var(--sx-line);
  margin: 8px 0;
}

/* 代码块：语言栏 + 复制按钮 + hljs 着色 */
.chat-text.md :deep(.md-code) {
  margin: 6px 0;
  border: 1px solid var(--sx-line);
  border-radius: 8px;
  overflow: hidden;
  background: #f6f8fa;
}
.chat-text.md :deep(.md-code-bar) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 2px 8px 2px 10px;
  border-bottom: 1px solid var(--sx-line);
  background: #eef1f4;
}
.chat-text.md :deep(.md-code-lang) {
  font-size: 11px;
  color: var(--sx-muted);
  font-family: var(--sx-font-mono);
}
.chat-text.md :deep(.md-copy-btn) {
  border: none;
  background: transparent;
  font-size: 11px;
  color: var(--sx-muted);
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
}
.chat-text.md :deep(.md-copy-btn:hover) {
  color: var(--sx-accent);
  background: rgba(63, 117, 171, 0.1);
}
.chat-text.md :deep(.md-code pre) {
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
}
.chat-text.md :deep(.md-code code) {
  background: transparent;
  padding: 0;
}
</style>
