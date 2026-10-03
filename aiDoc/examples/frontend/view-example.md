<!-- last-updated: 2026-10-03 -->
# 示例：前端列表页 view

> 目的：展示 `web/src/views/` 列表页的组织标准——SearchCard 查询区 + 远程分页表格 + 弹窗表单 + 权限点控制。

## 核心原则

1. `<script setup lang="ts">` + Naive UI 按需 import；文案全部 `t('...')` 走 i18n
2. 分页用 `usePagination` 组合函数，远程模式（`remote`）配 `{list, page}` 信封
3. 按钮/操作列权限：模板用 `v-permission="['<perm>']"`，渲染函数用 `userStore.has('<perm>')`

## 示例（提取自 web/src/views/system/Dicts.vue，节选）

```vue
<template>
  <SearchCard storage-key="dictTypes" @search="loadTypes(1)" @reset="resetQuery">
    <n-input v-model:value="query.name" :placeholder="t('dict.type.name')" clearable
      style="width: 200px" @keyup.enter="loadTypes(1)" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['dict:type:create']" type="primary" ghost @click="openTypeCreate">
          {{ t('dict.type.new') }}
        </n-button>
      </div>
    </template>
    <n-data-table size="small" :columns="typeColumns" :data="types"
      :loading="typesLoading" :pagination="pagination" remote :bordered="false" />
  </n-card>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { NDataTable, NTag, h, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { listDictTypes } from '../../api'
import type { DictType } from '../../api/types'

const { t } = useI18n()
const userStore = useUserStore()

const query = reactive({ name: '', code: '', page: 1, page_size: 20 })
const types = ref<DictType[]>([])
const typesLoading = ref(false)
const { pagination, setTotal } = usePagination(query, () => loadTypes(query.page))

async function loadTypes(page = 1) {
  typesLoading.value = true
  try {
    query.page = page
    const res = await listDictTypes({ page, page_size: query.page_size, name: query.name || undefined })
    types.value = res.data.data.list
    setTotal(res.data.data.page.total)
  } finally {
    typesLoading.value = false
  }
}

const typeColumns = computed<DataTableColumns<DictType>>(() => [
  { title: t('dict.type.name'), key: 'name', minWidth: 160 },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => {
      const actions: TableAction[] = []
      if (userStore.has('dict:type:update')) actions.push({ label: t('common.edit'), onClick: () => openTypeEdit(row) })
      if (userStore.has('dict:type:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmTypeDelete(row) })
      return renderActions(actions)
    },
  },
])
</script>
```

## 要点

- 查询条件与分页参数同放一个 `reactive` query 对象，`usePagination` 负责翻页状态
- 表格列用 `computed` 包裹（依赖 i18n 切换重渲染）；操作列经 `renderActions` 输出
- 新页面三件套别漏：`router/dynamic.ts` 登记 `menu:<code>`、locales 双语 key、菜单/权限点数据

## 真实参考文件

- `web/src/views/system/Dicts.vue`（完整页面：类型列表 + 项抽屉 + 双弹窗）
- `web/src/components/SearchCard.vue`、`web/src/utils/pagination.ts`、`web/src/utils/tableActions.ts`
