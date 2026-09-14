// 触发浏览器保存 blob（下载需带 JWT 的接口统一按 blob 取回后走这里）
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// 从 Content-Disposition 解析文件名：优先 RFC5987 的 filename*=UTF-8''…，退回 filename="…"；解析不到返回空串
export function parseDispositionFilename(disposition?: string): string {
  if (!disposition) return ''
  const star = /filename\*=(?:UTF-8|utf-8)''([^;]+)/.exec(disposition)
  if (star?.[1]) {
    try {
      return decodeURIComponent(star[1].trim())
    } catch {
      return star[1].trim()
    }
  }
  const plain = /filename="?([^";]+)"?/.exec(disposition)
  return plain?.[1]?.trim() ?? ''
}
