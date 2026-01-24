<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { UiCard } from '@/components/ui'
import { type CreatorVideoItem, VideoStatus } from '@/views/Creator/api/api'

const props = defineProps<{ video: CreatorVideoItem }>()

const emit = defineEmits<{ (e: 'edit', video: CreatorVideoItem): void }>()

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
	<UiCard class="flex gap-4 p-4 cursor-pointer" @click="handleClick">
		<div class="relative w-40 aspect-video overflow-hidden rounded bg-gray-200 flex-shrink-0">
			<img
				v-if="video.cover_url"
				:src="video.cover_url"
				alt="cover"
				class="h-full w-full object-cover"
			/>
			<div v-else class="h-full w-full flex items-center justify-center text-xs text-gray-400">
				暂无封面
			</div>
			<div v-if="durationText" class="absolute bottom-1 right-1 rounded bg-black/70 px-1.5 py-0.5 text-[10px] text-white">
				{{ durationText }}
			</div>
		</div>
		<div class="flex-1 flex flex-col justify-between gap-2 min-w-0">
			<div class="space-y-1">
				<div class="line-clamp-2 text-sm font-semibold text-gray-900">
					{{ video.title || '未命名视频' }}
				</div>
				<div v-if="video.description" class="line-clamp-2 text-xs text-gray-500">
					{{ video.description }}
				</div>
			</div>
			<div class="flex items-center justify-between text-[11px] text-gray-500">
				<div class="flex items-center gap-2">
					<span v-if="formattedDate">创建于 {{ formattedDate }}</span>
				</div>
				<div class="flex items-center gap-2">
					<span
						v-if="statusText"
						class="rounded-full border border-gray-300 px-2 py-0.5 text-[10px] text-gray-700 bg-gray-50"
					>
						{{ statusText }}
					</span>
					<button
						class="rounded border border-gray-300 px-2 py-0.5 text-[10px] text-gray-700 hover:bg-gray-50"
						@click="handleEditClick"
					>
						管理
					</button>
				</div>
			</div>
		</div>
	</UiCard>
</template>

<style scoped>
</style>
