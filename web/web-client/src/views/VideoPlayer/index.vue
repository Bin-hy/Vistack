<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import BiliLayout from '@/layouts/BiliLayout.vue'
import DashPlayer from '@/components/player-dash/DashPlayer.vue'
import { baseURL, get } from '@/lib/http'
import { getVideoSegmentsSignature, type VideoSegmentsSignatureCredentials } from './index'

interface VideoAuthor {
	id: string
	nickname: string
	avatar_url?: string
}

interface VideoInfo {
	id: string
	title: string
	description?: string | null
	created_at: string
	duration?: number
	user?: VideoAuthor | null
}

const route = useRoute()

const videoId = computed(() => route.params.id as string)

const manifestUrl = computed(() => {
	if (!videoId.value) return ''
	return `${baseURL}/videos/${videoId.value}/manifest.mpd`
})

const segmentsBaseUrl = ref('')
const segmentsCredentials = ref<VideoSegmentsSignatureCredentials | null>(null)
let refreshTimer: number | null = null

const videoInfo = ref<VideoInfo | null>(null)

const hasLiked = ref(false)
const hasFavorited = ref(false)

const likeIcon = computed(() => (hasLiked.value ? '/liked.png' : '/like-normal.png'))
const favoriteIcon = computed(() => (hasFavorited.value ? '/collected.png' : '/collect-normal.png'))
const forwardIcon = computed(() => '/forward-normal.png')

const formattedDate = computed(() => {
	if (!videoInfo.value?.created_at) return ''
	const d = new Date(videoInfo.value.created_at)
	if (Number.isNaN(d.getTime())) return videoInfo.value.created_at
	return d.toLocaleString('zh-CN', {
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
	})
})

async function loadSignature(id: string) {
	const resp = await getVideoSegmentsSignature(id)
	segmentsBaseUrl.value = resp.base_url
	segmentsCredentials.value = resp.credentials
	scheduleRefresh(resp.credentials.expiration, id)
}

async function loadVideoInfo(id: string) {
	const resp = await get<VideoInfo>(`/videos/${id}/info`)
	videoInfo.value = resp
}

function toggleLike() {
	hasLiked.value = !hasLiked.value
}

function toggleFavorite() {
	hasFavorited.value = !hasFavorited.value
}

function handleForward() {}

function scheduleRefresh(expiration: string, id: string) {
	if (refreshTimer !== null) {
		clearTimeout(refreshTimer)
		refreshTimer = null
	}
	const expMs = new Date(expiration).getTime()
	const now = Date.now()
	const target = expMs - 2 * 60 * 1000
	let delay = target - now
	if (!Number.isFinite(delay) || delay <= 0) {
		delay = 10 * 1000
	}
	refreshTimer = window.setTimeout(() => {
		loadSignature(id)
	}, delay)
}

onMounted(() => {
	if (videoId.value) {
		loadSignature(videoId.value)
		loadVideoInfo(videoId.value)
	}
})

watch(
	() => videoId.value,
	(id) => {
		if (id) {
			loadSignature(id)
			loadVideoInfo(id)
		}
	},
)

onBeforeUnmount(() => {
	if (refreshTimer !== null) {
		clearTimeout(refreshTimer)
		refreshTimer = null
	}
})
</script>

<template>
	<BiliLayout>
		<div class="flex items-start gap-6">
			<div class="flex-1 space-y-4">
				<div class="rounded-lg bg-white shadow-sm p-3">
					<DashPlayer
						v-if="manifestUrl && segmentsBaseUrl && segmentsCredentials"
						:src="manifestUrl"
						:autoplay="true"
						:segments-base-url="segmentsBaseUrl"
						:segments-credentials="segmentsCredentials"
					/>
				</div>
				<div class="rounded-lg bg-white shadow-sm p-4 space-y-2">
					<h1 class="text-lg font-semibold">
						{{ videoInfo?.title || '视频标题' }}
					</h1>
					<div class="flex items-center justify-between text-xs text-gray-500">
						<div>
							<span v-if="formattedDate">发布于 {{ formattedDate }}</span>
						</div>
					</div>
					<div class="mt-3 flex items-center gap-3 text-xs">
						<button
							class="flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 text-gray-700 hover:bg-gray-200"
							type="button"
							@click="toggleLike"
						>
							<img :src="likeIcon" alt="like" class="h-4 w-4" />
							<span>点赞</span>
						</button>
						<button
							class="flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 text-gray-700 hover:bg-gray-200"
							type="button"
							@click="toggleFavorite"
						>
							<img :src="favoriteIcon" alt="favorite" class="h-4 w-4" />
							<span>收藏</span>
						</button>
						<button
							class="flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 text-gray-700 hover:bg-gray-200"
							type="button"
							@click="handleForward"
						>
							<img :src="forwardIcon" alt="forward" class="h-4 w-4" />
							<span>转发</span>
						</button>
					</div>
					<p v-if="videoInfo?.description" class="mt-2 text-sm text-gray-700 whitespace-pre-line">
						{{ videoInfo.description }}
					</p>
				</div>
			</div>
			<div class="w-72 space-y-4">
				<div class="rounded-lg bg-white shadow-sm p-4">
					<div class="flex items-center gap-3">
						<div class="h-12 w-12 rounded-full bg-gray-200 overflow-hidden flex items-center justify-center text-xs text-gray-500">
							<img
								v-if="videoInfo?.user?.avatar_url"
								:src="videoInfo.user.avatar_url"
								alt="avatar"
								class="h-full w-full object-cover"
							/>
							<span v-else>UP</span>
						</div>
						<div class="flex-1 min-w-0">
							<div class="truncate text-sm font-medium text-gray-900">
								{{ videoInfo?.user?.nickname || '未命名UP主' }}
							</div>
							<div class="mt-2 flex items-center gap-2 text-xs">
								<button
									class="h-7 rounded-full bg-[#00A1D6] px-3 text-xs font-medium text-white hover:bg-[#0090c0]"
									type="button"
								>
									关注
								</button>
								<button
									class="h-7 rounded-full border border-gray-300 px-3 text-xs font-medium text-gray-700 hover:bg-gray-50"
									type="button"
								>
									发私信
								</button>
							</div>
						</div>
					</div>
				</div>
				<div class="rounded-lg bg-white shadow-sm p-4 text-xs text-gray-600">
					<div class="font-medium text-gray-800 mb-1">视频信息</div>
					<div>视频 ID：{{ videoId }}</div>
					<div v-if="videoInfo?.duration">时长：{{ Math.round(videoInfo.duration) }} 秒</div>
				</div>
			</div>
		</div>
	</BiliLayout>
</template>
