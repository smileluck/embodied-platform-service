<template>
  <div class="about-page">
    <!-- 固定左右布局：左信息卡固定宽、右更新记录自适应，任何窗口宽度都不堆叠 -->
    <div class="about-layout">
      <!-- 左：产品信息卡（品牌 + 系统简介 + 核心特性） -->
      <n-card class="info-card">
        <div class="brand">
          <div class="seal">S</div>
          <div class="brand-text">
            <span class="brand-name">SmileX Admin</span>
            <n-tag size="small" round type="primary">v{{ version }}</n-tag>
          </div>
        </div>

        <p class="intro">{{ t('about.intro') }}</p>

        <div class="features">
          <div class="features-title">{{ t('about.featuresTitle') }}</div>
          <div v-for="f in features" :key="f" class="feature-item">
            <span class="feature-dot" />
            <span>{{ f }}</span>
          </div>
        </div>

        <div class="copyright mono">© 2026 SmileX · internal use only</div>
      </n-card>

      <!-- 右：更新记录（git 提交日志构建时生成） -->
      <n-card class="log-card" :title="t('about.updateLog')">
        <template #header-extra>
          <span class="gen-meta mono">{{ t('about.commitMeta', { n: commits.length }) }}</span>
        </template>
        <div class="log-list">
          <div v-for="c in commits" :key="c.hash" class="log-item">
            <n-tag :type="typeMeta(c.type).tag" size="small" round>{{ typeMeta(c.type).label }}</n-tag>
            <span v-if="c.scope" class="log-scope mono">{{ c.scope }}</span>
            <span class="log-message">{{ c.message }}</span>
            <span class="log-date mono">{{ c.date }}</span>
          </div>
        </div>
      </n-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NCard, NTag } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import pkg from '../../../package.json'
import changelog from '../../generated/changelog.json'

const { t } = useI18n()
const version = pkg.version
const commits = changelog.commits

// 核心特性列表（文案见 about.features.*，与 README 特性说明同源）
const features = computed(() => [
  t('about.features.arch'),
  t('about.features.db'),
  t('about.features.rbac'),
  t('about.features.menu'),
  t('about.features.token'),
  t('about.features.i18n'),
])

// 提交类型 -> 标签色（conventional commits），文案见 about.type.*
type TagType = 'primary' | 'error' | 'info' | 'warning' | 'success' | 'default'
const TYPE_TAGS: Record<string, TagType> = {
  feat: 'primary',
  fix: 'error',
  style: 'info',
  refactor: 'warning',
  perf: 'success',
  docs: 'default',
  test: 'default',
  chore: 'default',
}
const typeMeta = (type: string) => ({
  tag: TYPE_TAGS[type] ?? ('default' as const),
  label: t(`about.type.${TYPE_TAGS[type] ? type : 'other'}`),
})
</script>

<style scoped>
/* 撑满一屏：100vh - 顶栏 64px - 内容区上下 padding 8px×2（height:100% 链在
   naive 嵌套滚动容器下不精确，会导致整页滚动；改用视口确定性高度） */
.about-layout {
  display: flex;
  gap: 12px;
  align-items: stretch;
  height: calc(100vh - 64px - 16px);
}
/* 左右 4:6 固定比例布局，任何窗口宽度都并排 */
.info-card {
  flex: 4 1 0;
  min-width: 0;
}
.log-card {
  flex: 6 1 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
/* 卡片内容纵向撑满，记录列表吃满剩余高度并内部滚动
   （naive 卡片内容类为 n-card-content，非 n-card__content） */
.log-card :deep(.n-card-content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
/* 左卡内容在极小高度下兜底滚动，避免溢出 */
.info-card :deep(.n-card-content) {
  overflow: auto;
}
/* 窄窗口下隐藏次要信息，保证并排布局可用 */
@media (max-width: 540px) {
  .log-date {
    display: none;
  }
}

/* 品牌区：印章 + 名称 + 版本 */
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--sx-line);
}
.seal {
  width: 46px;
  height: 46px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  font-family: var(--sx-font-mono);
  font-weight: 700;
  font-size: 22px;
  color: #fff;
  background: var(--sx-accent);
}
.brand-text {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}
.brand-name {
  font-size: 16px;
  font-weight: 700;
  color: var(--sx-ink);
}

/* 系统简介段落 */
.intro {
  margin: 14px 0 0;
  font-size: 13px;
  line-height: 1.8;
  color: var(--sx-ink);
}

/* 核心特性列表 */
.features {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--sx-line);
}
.features-title {
  font-size: 12px;
  color: var(--sx-muted);
  margin-bottom: 6px;
}
.feature-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 5px 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--sx-ink);
}
.feature-dot {
  flex-shrink: 0;
  width: 5px;
  height: 5px;
  margin-top: 7px;
  border-radius: 50%;
  background: var(--sx-accent);
}

.copyright {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--sx-line);
  font-size: 10px;
  color: var(--sx-muted);
}

/* 更新记录列表 */
.gen-meta {
  font-size: 11px;
  color: var(--sx-muted);
}
.log-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  scrollbar-width: thin;
}
.log-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 2px;
  border-bottom: 1px solid var(--sx-line);
}
.log-item:last-child {
  border-bottom: none;
}
.log-scope {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--sx-accent);
}
.log-message {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--sx-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.log-date {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--sx-muted);
}
</style>
