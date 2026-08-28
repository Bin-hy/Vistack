<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import BiliLayout from '@/layouts/BiliLayout.vue'
import { UiIcon } from '@/components/ui'
import { getVideoRecommend, type VideoItem } from './api/api'
import VideoCard from '@/components/video/VideoCard.vue'

const videos = ref<VideoItem[]>([])
const loading = ref(true)
const sortMode = ref<'recommend' | 'latest'>('recommend')

const sortedVideos = computed(() => {
  if (sortMode.value !== 'latest') return videos.value
  return [...videos.value].sort((a, b) => {
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })
})

async function loadData() {
  loading.value = true
  try {
    const res = await getVideoRecommend()
    videos.value = res.videos || []
  } catch (error) {
    console.error('load recommend failed', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <BiliLayout>
    <div class="animate-fade-in py-2">
      <!-- 顶部横幅 -->
      <div
        class="relative mb-6 overflow-hidden rounded-2xl border border-white/10 bg-gradient-to-br from-[hsl(var(--gradient-from))/0.25] via-transparent to-[hsl(var(--gradient-to))/0.2] p-6 sm:p-8"
      >
        <div class="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-[hsl(var(--gradient-to))/0.3] blur-3xl"></div>
        <div class="relative flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <div class="mb-2 inline-flex items-center gap-1.5 rounded-full border border-primary/30 bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
              <UiIcon name="sparkles" :size="14" /> 精选内容
            </div>
            <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">
              <span class="gradient-text">热门推荐</span>
            </h1>
            <p class="mt-1.5 max-w-md text-sm text-muted-foreground">
              精选优质视频内容，为你呈现每一帧的精彩。
            </p>
          </div>
          <div class="flex rounded-lg border border-white/10 bg-black/20 p-1">
            <button
              class="rounded-md px-4 py-1.5 text-sm font-medium transition-all"
              :class="sortMode === 'recommend' ? 'bg-accent text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="sortMode = 'recommend'"
            >
              推荐
            </button>
            <button
              class="rounded-md px-4 py-1.5 text-sm font-medium transition-all"
              :class="sortMode === 'latest' ? 'bg-accent text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="sortMode = 'latest'"
            >
              最新
            </button>
          </div>
        </div>
      </div>

      <!-- 内容区 -->
      <div class="mb-4 flex items-center gap-2">
        <span class="h-4 w-1 rounded-full bg-gradient-to-b from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))]"></span>
        <h2 class="text-lg font-semibold">{{ sortMode === 'latest' ? '最新发布' : '为你推荐' }}</h2>
      </div>

      <!-- 骨架屏 -->
      <div
        v-if="loading"
        class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
      >
        <div v-for="i in 10" :key="i" class="space-y-2.5">
          <div class="aspect-video rounded-xl bg-secondary bg-[linear-gradient(90deg,transparent,hsl(var(--glass)/0.08),transparent)] bg-[length:200%_100%] animate-shimmer"></div>
          <div class="flex gap-2.5">
            <div class="h-8 w-8 rounded-full bg-secondary"></div>
            <div class="flex-1 space-y-2">
              <div class="h-4 w-full rounded bg-secondary"></div>
              <div class="h-3 w-1/2 rounded bg-secondary"></div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-else-if="sortedVideos.length > 0"
        class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
      >
        <VideoCard v-for="item in sortedVideos" :key="item.id" :video="item" />
      </div>

      <div
        v-else
        class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border py-24 text-muted-foreground"
      >
        <UiIcon name="tv" :size="40" class="mb-3 opacity-40" />
        <p class="text-sm">暂无推荐内容</p>
      </div>
    </div>
  </BiliLayout>
</template>

<style scoped>
</style>
