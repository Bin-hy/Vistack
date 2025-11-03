<script setup lang="ts">
import BiliLayout from '@/layouts/BiliLayout.vue'

type Item = { title: string; cover: string; up: string; views: string }
const items: Item[] = Array.from({ length: 9 }).map((_, i) => ({
  title: `示例标题 ${i + 1} - 这是一个较长的标题用于占位展示`,
  cover: `https://picsum.photos/seed/${i + 1}/320/180`,
  up: '示例UP主',
  views: `${(Math.random() * 10 + 1).toFixed(1)}万`,
}))
</script>

<template>
  <BiliLayout>
    <!-- 卡片占位：模拟视频条目 -->
    <div
      v-for="item in items"
      :key="item.title"
      class="rounded-lg border border-[hsl(var(--border))] overflow-hidden bg-white/90 dark:bg-neutral-900/80"
    >
      <div class="h-40 bg-gray-100 dark:bg-neutral-800">
        <img :src="item.cover" alt="cover" class="w-full h-full object-cover" />
      </div>
      <div class="p-3">
        <h4 class="text-sm font-medium truncate">{{ item.title }}</h4>
        <p class="mt-1 text-xs text-gray-500">{{ item.up }} · {{ item.views }}次观看</p>
      </div>
    </div>
  </BiliLayout>
  
</template>

<style scoped>
</style>