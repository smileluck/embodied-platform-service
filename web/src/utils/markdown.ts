// 对话消息 markdown 渲染：html:false 防注入（原始 HTML 一律转义），
// 代码块经 highlight.js 高亮并附带语言标签与复制按钮（点击事件由容器委托处理）。
import 'highlight.js/styles/github.css'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/common'

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

// 复制按钮文案（由调用方按 locale 传入，默认英文）
let copyLabel = 'copy'

const md = new MarkdownIt({
  html: false, // 仅渲染 markdown 语法，原始 HTML 转义（存储型 XSS 防线）
  linkify: true,
  breaks: true, // 单换行视为 <br>，贴合聊天排版
  highlight(code, lang) {
    let body: string
    let label = lang || ''
    if (lang && hljs.getLanguage(lang)) {
      try {
        body = hljs.highlight(code, { language: lang, ignoreIllegals: true }).value
      } catch {
        body = escapeHtml(code)
      }
    } else {
      const auto = code.match(/^\s*</) ? undefined : hljs.highlightAuto(code) // 疑似 HTML 的跳过自动识别
      body = auto ? auto.value : escapeHtml(code)
      label = auto?.language || label
    }
    // 语言标签 + 复制按钮由容器级点击委托（.md-copy-btn）取 code 文本
    return (
      `<div class="md-code"><div class="md-code-bar"><span class="md-code-lang">${escapeHtml(label)}</span>` +
      `<button type="button" class="md-copy-btn">${escapeHtml(copyLabel)}</button></div>` +
      `<pre><code class="hljs">${body}</code></pre></div>`
    )
  },
})

// 链接统一新窗口打开并切断 opener（渲染层兜底，配合 CSP）；
// link_open 默认渲染即 renderToken，加完属性后直接复用
md.renderer.rules.link_open = (tokens, idx, options, _env, self) => {
  const a = tokens[idx]
  const i = a.attrIndex('target')
  if (i < 0) a.attrPush(['target', '_blank'])
  else a.attrs![i][1] = '_blank'
  const j = a.attrIndex('rel')
  if (j < 0) a.attrPush(['rel', 'noopener noreferrer'])
  else a.attrs![j][1] = 'noopener noreferrer'
  return self.renderToken(tokens, idx, options)
}

export function renderMarkdown(text: string, opts?: { copyLabel?: string }): string {
  if (opts?.copyLabel) copyLabel = opts.copyLabel
  return md.render(text || '')
}
