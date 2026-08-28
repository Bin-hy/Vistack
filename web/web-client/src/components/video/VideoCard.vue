<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { VideoItem } from '@/views/Index/api/api'
import { UiIcon } from '@/components/ui'

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
    <div class="card-hover relative aspect-video w-full overflow-hidden rounded-xl bg-secondary">
      <img
        v-if="video.cover_url"
        :src="video.cover_url"
        class="h-full w-full object-cover transition-transform duration-500 ease-out group-hover:scale-105"
        alt="cover"
        loading="lazy"
      />
      <div v-else class="flex h-full w-full items-center justify-center text-xs text-muted-foreground">
        暂无封面
      </div>

      <!-- Hover gradient -->
      <div
        class="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"
      ></div>

      <!-- Hover play overlay -->
      <div
        class="pointer-events-none absolute inset-0 flex items-center justify-center opacity-0 transition-all duration-300 group-hover:opacity-100"
      >
        <span
          class="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-white shadow-glow-sm transition-transform duration-300 group-hover:scale-100 scale-75"
        >
          <UiIcon name="play" :size="22" class="translate-x-px" />
        </span>
      </div>

      <!-- Duration Badge -->
      <div
        class="absolute bottom-2 right-2 rounded-md bg-black/70 px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-white backdrop-blur-sm"
      >
        {{ durationText }}
      </div>
    </div>

    <!-- Info -->
    <div class="mt-2.5 flex gap-2.5">
      <!-- Author avatar -->
      <img
        v-if="video.user?.avatar_url"
        :src="video.user.avatar_url"
        alt="author"
        class="h-8 w-8 shrink-0 rounded-full object-cover ring-1 ring-white/10"
        loading="lazy"
      />
      <div v-else class="h-8 w-8 shrink-0 rounded-full bg-secondary ring-1 ring-white/10"></div>

      <div class="min-w-0 space-y-1">
        <h3
          class="line-clamp-2 text-sm font-medium leading-snug text-foreground transition-colors group-hover:text-primary"
        >
          {{ video.title }}
        </h3>
        <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span v-if="video.user" class="truncate transition-colors hover:text-foreground">{{ video.user.nickname }}</span>
          <span v-if="video.user" class="text-muted-foreground/50">·</span>
          <span class="shrink-0">{{ formattedDate }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
