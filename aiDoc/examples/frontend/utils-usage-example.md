<!-- last-updated: 2026-10-03 -->
# 示例：前端工具函数使用

> 目的：展示 `web/src/utils/` 既有工具的标准用法——先复用、不自造。

## 核心原则

1. 任何「格式化/下载/分页/渲染」需求先查 [../../frontend/frontend-utils.md](../../frontend/frontend-utils.md) 清单
2. 工具函数集中在 `web/src/utils/<域>.ts`，页面内不写平行实现
3. 请求层已全局处理的（如 deepTrim 去空格），页面层不再重复

## 示例一：分页（usePagination，配远程表格）

```ts
import { usePagination } from '../../utils/pagination'

const query = reactive({ name: '', page: 1, page_size: 20 })
const { pagination, setTotal } = usePagination(query, () => load(query.page))

// load 内：res.data.data.list 赋表格数据，setTotal(res.data.data.page.total)
```

## 示例二：表格操作列（renderActions）

```ts
import { renderActions, type TableAction } from '../../utils/tableActions'

const columns = computed<DataTableColumns<Row>>(() => [
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => renderActions([
      { label: t('common.edit'), onClick: () => openEdit(row) },
      { label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) },
    ]),
  },
])
```

## 示例三：文件下载（saveBlob）

```ts
import { saveBlob } from '../../utils/download'

const blob = new Blob([content], { type: 'text/csv;charset=utf-8' })
saveBlob(blob, `export-${Date.now()}.csv`)
```

## 要点

- `formatDateTime` 统一时间展示；`renderMenuIcon` 统一菜单图标——页面禁止自造格式
- 全局 toast/错误提示由 `web/src/api/request.ts` 兜底，页面只处理自己 catch 到的 4xx 业务提示

## 真实参考文件

- `web/src/utils/pagination.ts`、`web/src/utils/tableActions.ts`、`web/src/utils/download.ts`
- 使用方：`web/src/views/system/Dicts.vue`、`web/src/views/file/Files.vue`
