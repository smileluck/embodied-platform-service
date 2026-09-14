<template>
  <div class="profile-page">
    <!-- 左：身份摘要 -->
    <n-card class="sum-card">
      <div class="sum">
        <div class="sum-avatar">{{ avatarChar }}</div>
        <div class="sum-name">{{ user?.nickname || user?.username }}</div>
        <div class="sum-username mono">@{{ user?.username }}</div>
        <div class="sum-roles">
          <n-tag v-for="r in user?.role_names || []" :key="r" size="small" round>{{ r }}</n-tag>
          <span v-if="!user?.role_names?.length" class="sum-empty">{{ t('profile.noRole') }}</span>
        </div>
        <div class="sum-meta">
          <div class="sum-meta-row">
            <span class="muted">{{ t('profile.email') }}</span>
            <span class="mono">{{ user?.email || '—' }}</span>
          </div>
          <div class="sum-meta-row">
            <span class="muted">{{ t('common.status') }}</span>
            <n-tag :type="user?.status === 1 ? 'success' : 'error'" size="small" round>
              {{ user?.status === 1 ? t('common.enabled') : t('common.disabled') }}
            </n-tag>
          </div>
          <div class="sum-meta-row">
            <span class="muted">{{ t('profile.joinTime') }}</span>
            <span class="mono">{{ user?.created_at || '—' }}</span>
          </div>
        </div>
      </div>
    </n-card>

    <!-- 右：资料编辑 -->
    <n-card class="info-card" :title="t('profile.basicInfo')">
      <n-form ref="infoFormRef" :model="infoForm" :rules="infoRules" label-placement="left" label-width="80" class="form">
        <n-form-item :label="t('profile.nickname')" path="nickname">
          <n-input v-model:value="infoForm.nickname" :maxlength="20" show-word-limit :placeholder="t('profile.nicknamePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('profile.email')" path="email">
          <n-input v-model:value="infoForm.email" :placeholder="t('profile.emailPlaceholder')" />
        </n-form-item>
        <div class="form-actions">
          <n-button type="primary" :loading="infoSaving" @click="saveInfo">{{ t('common.save') }}</n-button>
        </div>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NForm, NFormItem, NInput, NButton, NTag, useMessage,
  type FormInst, type FormRules,
} from 'naive-ui'
import { getProfile, updateProfile } from '../../api'
import { useUserStore } from '../../stores/user'

const message = useMessage()
const userStore = useUserStore()
const { t } = useI18n()

const user = computed(() => userStore.user)
const avatarChar = computed(() => (user.value?.nickname || user.value?.username || 'U').charAt(0).toUpperCase())

// 进入页面刷新一次本人信息（角色名/资料保持最新），并回填编辑表单
onMounted(async () => {
  try {
    const { data } = await getProfile()
    userStore.user = data.data.user
  } catch { /* 拉取失败沿用 store 现有数据 */ }
  const u = userStore.user
  if (u) {
    infoForm.nickname = u.nickname || ''
    infoForm.email = u.email || ''
  }
})

// ---- 基本信息 ----
const infoFormRef = ref<FormInst | null>(null)
const infoForm = reactive({ nickname: '', email: '' })
const infoSaving = ref(false)
// 与后端 binding 保持一致：昵称≤20、邮箱格式（选填）
const infoRules = computed<FormRules>(() => ({
  nickname: [{ max: 20, message: t('profile.rules.nicknameTooLong'), trigger: ['blur', 'input'] }],
  email: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule, value: string) => !value || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value),
      message: t('profile.rules.emailInvalid'),
    },
  ],
}))

async function saveInfo() {
  try {
    await infoFormRef.value?.validate()
  } catch {
    return // 校验失败，错误已在表单项上展示
  }
  infoSaving.value = true
  try {
    const { data } = await updateProfile({ nickname: infoForm.nickname.trim(), email: infoForm.email.trim() })
    userStore.user = data.data.user
    message.success(t('common.saveSuccess'))
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('profile.saveFailed'))
  } finally {
    infoSaving.value = false
  }
}
</script>

<style scoped>
/* 撑满一屏：100vh - 顶栏 64px - 内容区上下 padding 8px×2（height:100% 链在
   naive 嵌套滚动容器下不精确，会导致整页滚动；改用视口确定性高度） */
.profile-page {
  display: flex;
  gap: 12px;
  align-items: stretch;
  height: calc(100vh - 64px - 16px);
}
/* 左右 4:6 固定比例布局，任何窗口宽度都并排 */
.sum-card {
  flex: 4 1 0;
  min-width: 0;
}
.info-card {
  flex: 6 1 0;
  min-width: 0;
}
/* 卡片内容在极小高度下兜底滚动，避免溢出（naive 卡片内容类为 n-card-content） */
.sum-card :deep(.n-card-content),
.info-card :deep(.n-card-content) {
  overflow: auto;
}
.sum {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 8px 8px;
  text-align: center;
}
.sum-avatar {
  width: 64px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-weight: 700;
  font-size: 26px;
  background: var(--sx-accent);
  margin-bottom: 12px;
}
.sum-name {
  font-size: 17px;
  font-weight: 600;
  color: var(--sx-ink);
}
.sum-username {
  font-size: 12px;
  color: var(--sx-muted);
  margin: 2px 0 12px;
}
.sum-roles {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
  margin-bottom: 16px;
}
.sum-empty {
  font-size: 12px;
  color: var(--sx-muted);
}
.sum-meta {
  width: 100%;
  border-top: 1px solid var(--sx-line);
  padding-top: 12px;
}
.sum-meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 4px;
  font-size: 13px;
  color: var(--sx-ink);
}
.muted {
  color: var(--sx-muted);
}
.form {
  max-width: 420px;
}
.form-actions {
  padding-left: 80px;
}
</style>
