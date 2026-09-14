// changelog.mjs 构建前从 git 提交日志生成前端更新记录数据（src/generated/changelog.json）
// 由 package.json 的 predev/prebuild 钩子自动执行；无 git 环境时保留已有文件（构建不阻断）
import { execSync } from 'node:child_process'
import { mkdirSync, writeFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)))
const outDir = path.join(root, 'src', 'generated')
const outFile = path.join(outDir, 'changelog.json')

let raw
try {
  raw = execSync(
    "git log --no-merges --date=format:%Y-%m-%d --pretty=format:'%h|%ad|%s'",
    { maxBuffer: 16 * 1024 * 1024, cwd: root },
  ).toString()
} catch (e) {
  if (existsSync(outFile)) {
    console.log('changelog: git 不可用，保留已有生成文件')
    process.exit(0)
  }
  raw = ''
}

// 逐行解析：hash|date|subject（subject 内可能含 |，用 indexOf 切片）
const commits = raw.trim().split('\n').filter(Boolean).map((line) => {
  const i = line.indexOf('|')
  const j = line.indexOf('|', i + 1)
  const hash = line.slice(0, i)
  const date = line.slice(i + 1, j)
  const subject = line.slice(j + 1)
  // conventional commits：type(scope): message
  const m = subject.match(/^(\w+)(?:\(([^)]+)\))?:\s*(.+)$/)
  return {
    hash,
    date,
    type: m ? m[1] : 'other',
    scope: m ? m[2] || '' : '',
    message: m ? m[3] : subject,
  }
})

mkdirSync(outDir, { recursive: true })
writeFileSync(outFile, JSON.stringify({ generatedAt: new Date().toISOString(), commits }, null, 2) + '\n')
console.log(`changelog: 生成 ${commits.length} 条提交记录 -> src/generated/changelog.json`)
