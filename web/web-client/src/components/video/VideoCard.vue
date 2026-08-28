<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { VideoItem } from '@/views/Index/api/api'

const props = defineProps<{ video: VideoItem }>()
const router = useRouter()

const formattedDate = computed(() => {
  if (!props.video.created_at) return ''
  const d = new Date(props.video.created_at)
  const now = new Date()
  const diff = now.getTime() - d.getTime()

  if (diff < 60 * 1000) return '刚刚'
  if (diff < 60 * 60 * 1000) return `${Math.floor(diff / (60 * 1000))}分钟前`
  if (diff < 24 * 60 * 60 * 1000) return `${Math.floor(diff / (60 * 60 * 1000))}小时前`
  if (diff < 30 * 24 * 60 * 60 * 1000) return `${Math.floor(diff / (24 * 60 * 60 * 1000))}天前`

  return d.toLocaleDateString('zh-CN')
})

function formatDuration(seconds: number): string {
  if (!seconds) return '00:00'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  const mm = String(m).padStart(2, '0')
  const ss = String(s).padStart(2, '0')
  if (h > 0) {
    const hh = String(h).padStart(2, '0')
    return `${hh}:${mm}:${ss}`
  }
  return `${mm}:${ss}`
}

const durationText = computed(() => formatDuration(props.video.duration))

function handleClick() {
  router.push({ name: 'video-player', params: { id: props.video.id } })
}
</script>

<template>
  <div class="group cursor-pointer" @click="handleClick">
    <!-- Cover -->
    <div class="glass relative aspect-video w-full overflow-hidden rounded-lg">
      <img
        v-if="video.cover_url"
        :src="video.cover_url"
        class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
        alt="cover"
      />
      <div v-else class="flex h-full w-full items-center justify-center text-xs text-muted-foreground">
        暂无封面
      </div>

      <!-- Duration Badge -->
      <div class="absolute bottom-1.5 right-1.5 rounded bg-black/60 px-1.5 py-0.5 text-[10px] text-white">
        {{ durationText }}
      </div>
    </div>

    <!-- Info -->
    <div class="mt-2 space-y-1">
      <h3 class="line-clamp-2 text-sm font-medium leading-snug text-foreground transition-colors group-hover:text-primary">
        {{ video.title }}
      </h3>
      <div class="flex items-center gap-2 text-xs text-muted-foreground">
        <span v-if="video.user">{{ video.user.nickname }}</span>
        <span v-if="video.user">·</span>
        <span>{{ formattedDate }}</span>
      </div>
    </div>
  </div>
</template>
