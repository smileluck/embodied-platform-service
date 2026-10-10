// usePagination 列表页分页统一封装：
// 页码/页大小变化回调查询；itemCount 驱动页数推导，并在分页组件左侧显示总条数。
import { reactive } from 'vue'
import { useI18n } from 'vue-i18n'

// 每页条数可选项（与后端 page.sizeMax 默认上限 100 对齐）
export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]

export function usePagination(query: { page: number; page_size: number }, load: () => void) {
  const { t } = useI18n()
  const pagination = reactive({
    page: 1,
    // 初始选中态跟随各页面的 query.page_size（如字典类型页默认 20），避免
    // 选择器显示与实际请求条数不一致；0（全量拉取场景）回落默认 10
    pageSize: query.page_size > 0 ? query.page_size : 10,
    itemCount: 0,
    showSizePicker: true,
    pageSizes: PAGE_SIZE_OPTIONS,
    // 分页组件左侧前缀：总条数
    prefix: () => t('common.total', { n: pagination.itemCount }),
    onChange: (p: number) => {
      query.page = p
      load()
    },
    onUpdatePageSize: (s: number) => {
      // remote 模式下组件不回写传入对象：页大小需同步回 pagination（选择器选中态
      // 与页数推导才正确），并回到第一页——沿用旧页码会落在超范围页拿到空列表
      pagination.pageSize = s
      pagination.page = 1
      query.page_size = s
      query.page = 1
      load()
    },
  })
  // setTotal 查询返回后更新总条数（pageCount 由 itemCount/pageSize 自动推导）
  const setTotal = (total: number) => {
    pagination.itemCount = total
  }
  // runSearch 搜索按钮入口：条件变化后从第一页查起，
  // 避免停留在旧页码（超出过滤后结果集）拿到空列表、表现为"搜不到/刷新慢"
  const runSearch = () => {
    query.page = 1
    load()
  }
  return { pagination, setTotal, runSearch }
}
