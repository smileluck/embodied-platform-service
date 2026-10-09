<!-- 物模型只读浏览页：全量拉取节点后按 parent_id 组装树（layer: base→category→model→instance），
     kw 为客户端过滤（命中节点 + 其祖先保留树路径）；右侧展示节点详情与已发布版本。
     全部只读，无写操作；版本列表入口由 thingModel:version 权限点控制。 -->
<template>
  <SearchCard storage-key="thing-models" @search="onSearch" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('thingModel.kwPlaceholder')" clearable style="width: 220px" @keyup.enter="onSearch" />
    <n-select v-model:value="query.layer" :options="layerOptions" :placeholder="t('thingModel.layerLabel')" clearable style="width: 140px" />
  </SearchCard>

  <div class="tm-layout">
    <n-card :title="t('thingModel.nodeTree')" size="small" class="tm-tree-card">
      <n-spin :show="loading">
        <n-tree
          v-if="treeData.length"
          :data="treeData"
          :selected-keys="selectedKeys"
          :render-label="renderNodeLabel"
          :default-expand-all="!!query.kw"
          block-line
          @update:selected-keys="onSelect"
        />
        <n-empty v-else-if="!loading" :description="t('common.noData')" />
      </n-spin>
    </n-card>

    <n-card :title="t('thingModel.nodeDetail')" size="small" class="tm-detail-card">
      <n-empty v-if="!selectedNode" :description="t('thingModel.selectNode')" />
      <template v-else>
        <n-descriptions :column="2" label-placement="left" size="small" bordered>
          <n-descriptions-item :label="t('thingModel.code')">{{ selectedNode.code }}</n-descriptions-item>
          <n-descriptions-item :label="t('thingModel.name')">{{ selectedNode.name }}</n-descriptions-item>
          <n-descriptions-item :label="t('thingModel.layerLabel')">
            <n-tag size="small" :bordered="false">{{ layerText(selectedNode.layer) }}</n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('common.status')">
            <n-tag size="small" :type="selectedNode.status === 1 ? 'success' : 'error'">
              {{ selectedNode.status === 1 ? t('common.enabled') : t('common.disabled') }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('thingModel.scope')">
            <n-tag size="small" :type="selectedNode.merchant_id === 0 ? 'default' : 'info'">
              {{ selectedNode.merchant_id === 0 ? t('thingModel.scopeCommon') : t('thingModel.scopeMerchant') }}
            </n-tag>
          </n-descriptions-item>
        </n-descriptions>

        <!-- 已发布版本（thingModel:version 权限点控制入口） -->
        <n-divider style="margin: 16px 0 12px">{{ t('thingModel.versions') }}</n-divider>
        <template v-if="userStore.has('thingModel:version')">
          <n-spin :show="versionsLoading">
            <n-data-table
              :columns="versionColumns" :data="versions" size="small" :pagination="false" :bordered="false"
            />
          </n-spin>
        </template>
        <n-alert v-else type="default" :show-icon="false">{{ t('thingModel.noVersionPerm') }}</n-alert>
      </template>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, type VNodeChild } from 'vue'
import {
  NAlert, NCard, NDataTable, NDescriptions, NDescriptionsItem, NDivider, NEmpty, NInput, NSelect,
  NSpin, NTag, NTree, useMessage, type DataTableColumns, type TreeOption,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { useUserStore } from '../../stores/user'
import { listThingModelNodes, listThingModelVersions } from '../../api'
import { formatDateTime } from '../../utils/datetime'
import type { TMNode, TMVersion } from '../../api/types'

const { t } = useI18n()
const userStore = useUserStore()
const message = useMessage()

const LAYER_ORDER = ['base', 'category', 'model', 'instance']

const layerOptions = computed(() =>
  LAYER_ORDER.map((l) => ({ label: t(`thingModel.layer.${l}`), value: l })),
)

function layerText(layer: string): string {
  return LAYER_ORDER.includes(layer) ? t(`thingModel.layer.${layer}`) : layer
}

// ---- 节点树 ----
const loading = ref(false)
const allNodes = ref<TMNode[]>([])
const query = reactive({ kw: '', layer: null as string | null })

// kw 客户端过滤：命中节点 + 全部祖先（保留树路径）；layer 精确过滤
const visibleNodes = computed(() => {
  let nodes = allNodes.value
  if (query.layer) nodes = nodes.filter((n) => n.layer === query.layer)
  const kw = query.kw.trim().toLowerCase()
  if (!kw) return nodes
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const keep = new Set<number>()
  for (const n of nodes) {
    if (!n.name.toLowerCase().includes(kw) && !n.code.toLowerCase().includes(kw)) continue
    let cur: TMNode | undefined = n
    while (cur && !keep.has(cur.id)) {
      keep.add(cur.id)
      cur = byId.get(cur.parent_id)
    }
  }
  return nodes.filter((n) => keep.has(n.id))
})

const treeData = computed<TreeOption[]>(() => {
  const nodes = visibleNodes.value
  const ids = new Set(nodes.map((n) => n.id))
  const byParent = new Map<number, TMNode[]>()
  for (const n of nodes) {
    const arr = byParent.get(n.parent_id) ?? []
    arr.push(n)
    byParent.set(n.parent_id, arr)
  }
  const build = (parentId: number): TreeOption[] =>
    (byParent.get(parentId) ?? [])
      .slice()
      .sort((a, b) => LAYER_ORDER.indexOf(a.layer) - LAYER_ORDER.indexOf(b.layer) || a.id - b.id)
      .map((n) => ({ key: n.id, label: `${n.name}（${n.code}）`, children: build(n.id), node: n }))
  // 根=parent_id 不在当前集合内的节点（parent_id=0 或父节点被过滤）
  return nodes
    .filter((n) => !ids.has(n.parent_id))
    .sort((a, b) => LAYER_ORDER.indexOf(a.layer) - LAYER_ORDER.indexOf(b.layer) || a.id - b.id)
    .map((n) => ({ key: n.id, label: `${n.name}（${n.code}）`, children: build(n.id), node: n }))
})

function renderNodeLabel({ option }: { option: TreeOption }): VNodeChild {
  const node = (option as any).node as TMNode
  return h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, [
    h('span', option.label as string),
    h(NTag, { size: 'tiny', bordered: false }, { default: () => layerText(node.layer) }),
    h(
      NTag,
      { size: 'tiny', bordered: false, type: node.merchant_id === 0 ? 'default' : 'info' },
      { default: () => (node.merchant_id === 0 ? t('thingModel.scopeCommon') : t('thingModel.scopeMerchant')) },
    ),
  ])
}

async function load() {
  loading.value = true
  try {
    const { data: resp } = await listThingModelNodes()
    allNodes.value = resp.data || []
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

function onSearch() {
  // 全量已拉取，搜索仅触发客户端过滤（computed 即时生效）
  selectedKeys.value = []
}

function resetQuery() {
  query.kw = ''
  query.layer = null
}

// ---- 节点详情与已发布版本 ----
const selectedKeys = ref<number[]>([])
const selectedNode = computed(() => allNodes.value.find((n) => n.id === selectedKeys.value[0]) ?? null)
const versions = ref<TMVersion[]>([])
const versionsLoading = ref(false)

const versionColumns = computed<DataTableColumns<TMVersion>>(() => [
  { title: t('thingModel.version'), key: 'version', width: 120, render: (v) => `v${v.version}` },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: () => h(NTag, { size: 'small', type: 'success' }, { default: () => t('thingModel.published') }),
  },
  {
    title: t('thingModel.publishedAt'), key: 'published_at', minWidth: 160,
    render: (v) => formatDateTime(v.published_at) || '—',
  },
])

async function onSelect(keys: Array<string | number>) {
  selectedKeys.value = keys as number[]
  versions.value = []
  const node = selectedNode.value
  if (!node || !userStore.has('thingModel:version')) return
  versionsLoading.value = true
  try {
    const { data: resp } = await listThingModelVersions(node.id)
    versions.value = resp.data || []
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    versionsLoading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.tm-layout {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}
.tm-tree-card {
  width: 420px;
  flex-shrink: 0;
}
.tm-detail-card {
  flex: 1;
  min-width: 0;
}
</style>
