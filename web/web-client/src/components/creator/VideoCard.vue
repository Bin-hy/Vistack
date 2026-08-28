<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { UiCard } from '@/components/ui'
import { type CreatorVideoItem, VideoStatus } from '@/views/Creator/api/api'

const props = defineProps<{ video: CreatorVideoItem }>()

const emit = defineEmits<{
  (e: 'edit', video: CreatorVideoItem): void
  (e: 'delete', video: CreatorVideoItem): void
}>()

const router = useRouter()

const formattedDate = computed(() => {
	if (!props.video.created_at) return ''
	const d = new Date(props.video.created_at)
	if (Number.isNaN(d.getTime())) return props.video.created_at
	return d.toLocaleString('zh-CN', {
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
	})
})

function formatDuration(seconds: number | undefined | null): string {
	if (!seconds || seconds <= 0) return ''
	const total = Math.floor(seconds)
	const h = Math.floor(total / 3600)
	const m = Math.floor((total % 3600) / 60)
	const s = total % 60
	const mm = String(m).padStart(2, '0')
	const ss = String(s).padStart(2, '0')
	if (h > 0) {
		const hh = String(h).padStart(2, '0')
		return `${hh}:${mm}:${ss}`
	}
	return `${mm}:${ss}`
}

const durationText = computed(() => formatDuration(props.video.duration as unknown as number))

const statusText = computed(() => {
	const val = props.video.status
	if (!val) return ''
	if (val === VideoStatus.Processing) return '转码中'
	if (val === VideoStatus.Published) return '已发布'
	if (val === VideoStatus.Uploaded) return '已上传'
	return val
})

const isPublished = computed(() => props.video.status === VideoStatus.Published)

function handleClick() {
	if (!isPublished.value) return
	router.push({ name: 'video-player', params: { id: props.video.id } })
}

function handleEditClick(event: MouseEvent) {
	event.stopPropagation()
	emit('edit', props.video)
}
</script>

<template>
	<UiCard class="flex cursor-pointer gap-3 p-3 transition-shadow hover:shadow-lg md:gap-4 md:p-4" @click="handleClick">
		<div class="relative aspect-video w-32 flex-shrink-0 overflow-hidden rounded bg-secondary md:w-40">
			<img
				v-if="video.cover_url"
				:src="video.cover_url"
				alt="cover"
				class="h-full w-full object-cover"
			/>
			<div v-else class="flex h-full w-full items-center justify-center text-xs text-muted-foreground">
				暂无封面
			</div>
			<div v-if="durationText" class="absolute bottom-1 right-1 rounded bg-black/70 px-1.5 py-0.5 text-[10px] text-white">
				{{ durationText }}
			</div>
		</div>
		<div class="flex min-w-0 flex-1 flex-col justify-between gap-1 md:gap-2">
			<div class="space-y-1">
				<div class="line-clamp-2 text-sm font-semibold leading-tight text-foreground">
					{{ video.title || '未命名视频' }}
				</div>
				<div v-if="video.description" class="line-clamp-1 text-xs text-muted-foreground md:line-clamp-2">
					{{ video.description }}
				</div>
			</div>
			<div class="flex flex-col justify-between gap-2 text-[11px] text-muted-foreground md:flex-row md:items-center md:gap-0">
				<div class="flex items-center gap-2">
					<span v-if="formattedDate" class="hidden sm:inline">创建于 {{ formattedDate }}</span>
					<span v-if="formattedDate" class="sm:hidden">{{ formattedDate.split(' ')[0] }}</span>
				</div>
				<div class="flex w-full items-center justify-between gap-2 md:w-auto md:justify-end">
					<span
						v-if="statusText"
						class="whitespace-nowrap rounded-full border border-border bg-secondary px-2 py-0.5 text-[10px] text-muted-foreground"
					>
						{{ statusText }}
					</span>
					<div class="flex items-center gap-2">
						<button
							class="rounded px-2 py-1 font-medium text-primary hover:bg-accent"
							@click="handleEditClick"
						>
							编辑
						</button>
						<button
							class="rounded px-2 py-1 font-medium text-destructive hover:bg-destructive/10"
							@click.stop="$emit('delete', video)"
						>
							删除
						</button>
					</div>
				</div>
			</div>
		</div>
	</UiCard>
</template>

<style scoped>
</style>
