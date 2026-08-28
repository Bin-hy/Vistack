<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import BiliLayout from '@/layouts/BiliLayout.vue'
import DashPlayer from '@/components/player-dash/DashPlayer.vue'
import { UiIcon } from '@/components/ui'
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
		<div class="flex flex-col items-start gap-4 lg:flex-row lg:gap-6">
			<!-- Main Content: Player + Info -->
			<div class="w-full space-y-4 lg:flex-1">
				<div class="glass overflow-hidden rounded-lg p-0 md:p-3">
					<DashPlayer
						v-if="manifestUrl && segmentsBaseUrl && segmentsCredentials"
						:src="manifestUrl"
						:autoplay="true"
						:segments-base-url="segmentsBaseUrl"
						:segments-credentials="segmentsCredentials"
					/>
				</div>
				<div class="glass space-y-2 rounded-lg p-4">
					<h1 class="text-lg font-semibold">
						{{ videoInfo?.title || '视频标题' }}
					</h1>
					<div class="flex items-center justify-between text-xs text-muted-foreground">
						<div>
							<span v-if="formattedDate">发布于 {{ formattedDate }}</span>
						</div>
					</div>
					<div class="mt-3 flex items-center gap-2 text-xs">
						<button
							class="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
							:class="{ 'text-primary': hasLiked }"
							type="button"
							@click="toggleLike"
						>
							<UiIcon name="thumbs-up" :size="16" :class="hasLiked ? 'fill-current' : ''" />
							<span>点赞</span>
						</button>
						<button
							class="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
							:class="{ 'text-primary': hasFavorited }"
							type="button"
							@click="toggleFavorite"
						>
							<UiIcon name="bookmark" :size="16" :class="hasFavorited ? 'fill-current' : ''" />
							<span>收藏</span>
						</button>
						<button
							class="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
							type="button"
							@click="handleForward"
						>
							<UiIcon name="share" :size="16" />
							<span>转发</span>
						</button>
					</div>
					<div class="border-t border-border pt-3 text-sm leading-relaxed text-foreground">
						{{ videoInfo?.description || '暂无简介' }}
					</div>
				</div>
			</div>

			<!-- Sidebar: Author + Rec -->
			<div class="w-full space-y-4 lg:w-80">
				<div class="glass rounded-lg p-4">
					<div class="flex items-center gap-3">
						<img
							:src="videoInfo?.user?.avatar_url || 'https://api.dicebear.com/7.x/avataaars/svg?seed=Felix'"
							alt="avatar"
							class="h-12 w-12 rounded-full border border-border"
						/>
						<div>
							<div class="font-medium text-foreground">
								{{ videoInfo?.user?.nickname || '未知UP主' }}
							</div>
							<div class="text-xs text-muted-foreground">这家伙很懒，什么都没写</div>
						</div>
					</div>
					<div class="mt-4 flex gap-2">
						<button
							class="flex-1 rounded-md bg-gradient-to-r from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] py-1.5 text-sm font-medium text-white hover:opacity-90"
						>
							关注
						</button>
						<button class="glass flex-1 rounded-md py-1.5 text-sm text-foreground hover:bg-accent">
							私信
						</button>
					</div>
				</div>

				<!-- Recommendations (Placeholder) -->
				<div class="glass rounded-lg p-4">
					<h3 class="mb-3 font-medium text-foreground">相关推荐</h3>
					<div class="space-y-3">
						<div v-for="i in 5" :key="i" class="flex gap-2">
							<div class="h-16 w-28 flex-shrink-0 rounded bg-secondary"></div>
							<div class="flex flex-col justify-between py-0.5">
								<div class="line-clamp-2 text-sm font-medium text-foreground">推荐视频标题演示内容...</div>
								<div class="text-xs text-muted-foreground">UP主名称</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>
	</BiliLayout>
</template>
