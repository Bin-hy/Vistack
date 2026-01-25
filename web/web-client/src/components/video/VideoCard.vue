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
  <div class="group cursor-pointer flex flex-col gap-2" @click="handleClick">
    <!-- Cover -->
    <div class="relative w-full aspect-video rounded-lg overflow-hidden bg-gray-100">
      <img 
        v-if="video.cover_url" 
        :src="video.cover_url" 
        class="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
        alt="cover"
      />
      <div v-else class="w-full h-full flex items-center justify-center text-gray-400 text-xs">
        暂无封面
      </div>
      
      <!-- Duration Badge -->
      <div class="absolute bottom-1.5 right-1.5 px-1.5 py-0.5 rounded bg-black/60 text-white text-[10px]">
        {{ durationText }}
      </div>
    </div>
    
    <!-- Info -->
    <div class="flex gap-3 items-start">
        <div class="flex-1 min-w-0 space-y-1">
            <h3 class="text-sm font-medium text-gray-900 line-clamp-2 leading-snug group-hover:text-pink-600 transition-colors">
                {{ video.title }}
            </h3>
            <div class="flex items-center text-xs text-gray-500 gap-2">
                <span v-if="video.user" class="flex items-center gap-1 hover:text-pink-500">
                    {{ video.user.nickname }}
                </span>
                <span v-if="video.user">·</span>
                <span>{{ formattedDate }}</span>
            </div>
        </div>
    </div>
  </div>
</template>
