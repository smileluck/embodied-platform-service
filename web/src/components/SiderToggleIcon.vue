<!-- 侧边栏折叠图标（自绘 SVG）：
     三条横线，中间行的箭头随折叠状态滑动翻转——
     展开时箭头在中间行左端指向左，折叠时滑到右端指向右（镜像变换，过渡与侧栏宽度动画同节奏）。 -->
<template>
  <svg viewBox="0 0 24 24" width="20" height="20" fill="none" aria-hidden="true" :class="{ collapsed }">
    <path class="ln" d="M4 6H20" />
    <path class="ln" d="M4 18H20" />
    <g class="mid">
      <path class="ln" d="M9 12H20" />
      <path class="ln" d="M9 8L5 12L9 16" />
    </g>
  </svg>
</template>

<script setup lang="ts">
defineProps<{ collapsed?: boolean }>()
</script>

<style scoped>
.ln {
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.mid {
  /* 镜像中心取画布中心：scaleX(-1) 后箭头平移到右端且指向右，中横线段对称换边 */
  transform-box: view-box;
  transform-origin: 12px 12px;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
svg.collapsed .mid {
  transform: scaleX(-1);
}
</style>
