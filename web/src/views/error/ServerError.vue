<template>
  <div class="err-page">
    <div class="err-code mono">500</div>
    <div class="err-title">{{ t('errorPage.serverErrorTitle') }}</div>
    <div class="err-desc">{{ t('errorPage.serverErrorDesc') }}</div>
    <n-space :size="10">
      <n-button @click="reload">{{ t('errorPage.reloadRetry') }}</n-button>
      <n-button type="primary" @click="goHome">{{ t('errorPage.backHome') }}</n-button>
    </n-space>
  </div>
</template>

<script setup lang="ts">
import { NButton, NSpace } from 'naive-ui'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const { t } = useI18n()

function reload() {
  // 保留当前地址整页重载（带时间戳绕过缓存兜底）
  window.location.reload()
}

function goHome() {
  router.push('/')
}
</script>

<style scoped>
.err-page {
  height: 100%;
  min-height: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  text-align: center;
}
.err-code {
  font-size: 72px;
  font-weight: 700;
  line-height: 1;
  color: var(--sx-danger);
  opacity: 0.85;
  margin-bottom: 10px;
}
.err-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--sx-ink);
}
.err-desc {
  font-size: 12px;
  color: var(--sx-muted);
  margin-bottom: 18px;
}
</style>
