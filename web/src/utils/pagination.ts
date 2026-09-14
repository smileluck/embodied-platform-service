// usePagination 列表页分页统一封装：
// 页码/页大小变化回调查询；itemCount 驱动页数推导，并在分页组件左侧显示总条数。
import { reactive } from 'vue'
import { useI18n } from 'vue-i18n'

export function usePagination(query: { page: number; page_size: number }, load: () => void) {
  const { t } = useI18n()
  const pagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: true,
    // 分页组件左侧前缀：总条数
    prefix: () => t('common.total', { n: pagination.itemCount }),
    onChange: (p: number) => {
      query.page = p
      load()
    },
    onUpdatePageSize: (s: number) => {
      query.page_size = s
      load()
    },
  })
  // setTotal 查询返回后更新总条数（pageCount 由 itemCount/pageSize 自动推导）
  const setTotal = (total: number) => {
    pagination.itemCount = total
  }
  return { pagination, setTotal }
}
