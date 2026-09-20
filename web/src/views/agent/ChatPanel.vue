<template>
  <!-- Agent 聊天面板：无状态调试对话（SSE 流式），供 调试抽屉 与 聊天测试页 复用 -->
  <div class="chat-panel">
    <div class="chat-head">
      <span class="chat-model mono">{{ currentModel || '—' }}</span>
      <span class="chat-sub">{{ t('agent.playground.subtitle') }}</span>
    </div>
    <div ref="listRef" class="chat-list">
      <div v-if="!messages.length" class="chat-empty">{{ t('agent.playground.empty') }}</div>
      <div v-for="(m, i) in messages" :key="i" class="chat-msg" :class="m.role">
        <div class="chat-bubble">
          <div class="chat-text">{{ m.content || (m.role === 'assistant' && m.streaming ? t('agent.playground.streaming') : '') }}</div>
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
import { computed, nextTick, ref, watch } from 'vue'
import { NButton, NInput } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useUserStore } from '../../stores/user'

interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
  error?: string
  usage?: { total_tokens: number }
}

const props = withDefaults(defineProps<{
  agentId: number
  modelLabel?: string // 初始模型标签（后续以 SSE meta 帧为准）
  rows?: number // 输入框行数（页面场景更高）
}>(), { modelLabel: '', rows: 3 })

const { t, locale } = useI18n()
const userStore = useUserStore()

const canChat = computed(() => userStore.has('agent:chat'))

const messages = ref<ChatMsg[]>([])
const input = ref('')
const sending = ref(false)
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

function clear() {
  messages.value = []
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
      body: JSON.stringify({ messages: history }),
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
</style>
