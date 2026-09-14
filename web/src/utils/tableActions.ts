import { h, type VNode } from 'vue'
import { NButton } from 'naive-ui'

/** 表格操作项：文字按钮描述；accent 主操作用主题色，danger 危险操作用警示色 */
export interface TableAction {
  label: string
  onClick?: () => void
  accent?: boolean
  danger?: boolean
}

const DIVIDER_STYLE = 'display:inline-block;width:1px;height:12px;background:var(--sx-line);flex-shrink:0'

/**
 * 表格「操作」列统一渲染：小号文字按钮 + 竖向发丝分隔线，替代彩色实心按钮堆叠。
 * items 可混入 VNode（如 NTag），与文字按钮同排。
 * 空数组渲染占位「—」。
 */
export function renderActions(items: Array<TableAction | VNode>): VNode {
  if (items.length === 0) {
    return h('span', { style: 'color: var(--sx-muted); font-size: 12px' }, '—')
  }
  const children: VNode[] = []
  items.forEach((item, i) => {
    if (i > 0) children.push(h('i', { style: DIVIDER_STYLE }))
    if (typeof item === 'object' && item !== null && 'label' in item) {
      const a = item as TableAction
      children.push(h(
        NButton,
        {
          text: true,
          size: 'small',
          type: a.danger ? 'error' : a.accent ? 'primary' : 'default',
          onClick: a.onClick,
          style: 'font-size:13px',
        },
        { default: () => a.label },
      ))
    } else {
      children.push(item as VNode)
    }
  })
  return h(
    'span',
    { style: 'display:inline-flex;align-items:center;gap:10px;white-space:nowrap' },
    children,
  )
}
