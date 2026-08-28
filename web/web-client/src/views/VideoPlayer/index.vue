<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BiliLayout from '@/layouts/BiliLayout.vue'
import DashPlayer, { type DanmakuItem } from '@/components/player-dash/DashPlayer.vue'
import { UiIcon } from '@/components/ui'
import { baseURL, get, post } from '@/lib/http'
import { formatCount } from '@/lib/format'
import { useUserStore } from '@/stores/user'
import { toast } from '@/components/ui/toast/useToast'
import { getVideoSegmentsSignature, type VideoSegmentsSignatureCredentials } from './index'
import { getVideoRecommend, type VideoItem } from '@/views/Index/api/api'

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
const router = useRouter()
const userStore = useUserStore()

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

const stats = ref<{ like_count: number; favorite_count: number; play_count: number }>({
	like_count: 0,
	favorite_count: 0,
	play_count: 0,
})

const relatedVideos = ref<VideoItem[]>([])
const relatedLoading = ref(false)

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

function formatDuration(seconds: number | undefined): string {
	if (!seconds) return '00:00'
	const h = Math.floor(seconds / 3600)
	const m = Math.floor((seconds % 3600) / 60)
	const s = seconds % 60
	const mm = String(m).padStart(2, '0')
	const ss = String(s).padStart(2, '0')
	if (h > 0) return `${String(h).padStart(2, '0')}:${mm}:${ss}`
	return `${mm}:${ss}`
}

function formatRelativeDate(value: string): string {
	const d = new Date(value)
	if (Number.isNaN(d.getTime())) return ''
	const diff = Date.now() - d.getTime()
	if (diff < 60 * 60 * 1000) return `${Math.max(1, Math.floor(diff / (60 * 1000)))}分钟前`
	if (diff < 24 * 60 * 60 * 1000) return `${Math.floor(diff / (60 * 60 * 1000))}小时前`
	if (diff < 30 * 24 * 60 * 60 * 1000) return `${Math.floor(diff / (24 * 60 * 60 * 1000))}天前`
	return d.toLocaleDateString('zh-CN')
}

async function loadSignature(id: string) {
	const resp = await getVideoSegmentsSignature(id)
	segmentsBaseUrl.value = resp.base_url
	segmentsCredentials.value = resp.credentials
	scheduleRefresh(resp.credentials.expiration, id)
}

async function loadVideoInfo(id: string) {
	const resp = await get<VideoInfo>(`/videos/${id}/info`)
	videoInfo.value = resp
	loadDanmaku(id, resp.duration || 300)
}

const danmakus = ref<DanmakuItem[]>([])

async function loadDanmaku(id: string, duration: number) {
	try {
		const end = Math.max(60, Math.ceil(duration || 0))
		const resp = await get<{ danmaku: DanmakuItem[] }>(`/videos/${id}/danmaku?start=0&end=${end}`)
		danmakus.value = resp.danmaku || []
	} catch {
		danmakus.value = []
	}
}

async function handleSendDanmaku(text: string, timeOffset: number) {
	if (!videoId.value) return
	if (!userStore.isLoggedIn) {
		toast({ title: '请先登录', type: 'error' })
		return
	}
	try {
		await post(`/videos/${videoId.value}/danmaku`, { content: text, time_offset: timeOffset })
	} catch (e: any) {
		toast({ title: e?.message ?? '发送失败', type: 'error' })
	}
}

async function loadRelated() {
	relatedLoading.value = true
	try {
		const res = await getVideoRecommend()
		relatedVideos.value = (res.videos || []).filter(v => v.id !== videoId.value).slice(0, 6)
	} catch {
		relatedVideos.value = []
	} finally {
		relatedLoading.value = false
	}
}

async function loadStats(id: string) {
	try {
		const resp = await get<{ like_count: number; favorite_count: number; play_count: number }>(`/videos/${id}/stats`)
		stats.value = resp
	} catch {
		// 静默忽略
	}
}

async function loadInteraction(id: string) {
	try {
		const resp = await get<{ liked: boolean; favorited: boolean }>(`/videos/${id}/interaction`)
		hasLiked.value = resp.liked
		hasFavorited.value = resp.favorited
	} catch {
		// 未登录时静默忽略
	}
}

async function reportPlay(id: string) {
	try {
		const resp = await post<{ play_count: number }>(`/videos/${id}/play`)
		stats.value.play_count = resp.play_count
	} catch {
		// 静默忽略
	}
}

function toggleLike() {
	if (!videoId.value) return
	if (!userStore.isLoggedIn) {
		toast({ title: '请先登录', type: 'error' })
		return
	}
	post<{ liked: boolean; like_count: number }>(`/videos/${videoId.value}/like`)
		.then((resp) => {
			hasLiked.value = resp.liked
			stats.value.like_count = resp.like_count
		})
		.catch(() => {})
}

function toggleFavorite() {
	if (!videoId.value) return
	if (!userStore.isLoggedIn) {
		toast({ title: '请先登录', type: 'error' })
		return
	}
	post<{ favorited: boolean; favorite_count: number }>(`/videos/${videoId.value}/favorite`)
		.then((resp) => {
			hasFavorited.value = resp.favorited
			stats.value.favorite_count = resp.favorite_count
		})
		.catch(() => {})
}

function handleForward() {}

function goVideo(id: string) {
	router.push({ name: 'video-player', params: { id } })
}

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
		loadRelated()
		loadStats(videoId.value)
		loadInteraction(videoId.value)
		reportPlay(videoId.value)
	}
})

watch(
	() => videoId.value,
	(id) => {
		if (id) {
			videoInfo.value = null
			loadSignature(id)
			loadVideoInfo(id)
			loadRelated()
			loadStats(id)
			loadInteraction(id)
			reportPlay(id)
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
		<div class="animate-fade-in flex flex-col items-start gap-4 lg:flex-row lg:gap-6">
			<!-- Main Content: Player + Info -->
			<div class="w-full space-y-4 lg:min-w-0 lg:flex-1">
				<div class="glass overflow-hidden rounded-2xl p-0 md:p-2">
					<DashPlayer
						v-if="manifestUrl && segmentsBaseUrl && segmentsCredentials"
						:src="manifestUrl"
						:autoplay="true"
						:segments-base-url="segmentsBaseUrl"
						:segments-credentials="segmentsCredentials"
						:danmakus="danmakus"
						@send-danmaku="handleSendDanmaku"
					/>
				</div>

				<!-- 视频信息 -->
				<div class="glass space-y-3 rounded-2xl p-4 sm:p-5">
					<h1 class="text-lg font-semibold leading-snug sm:text-xl">
						{{ videoInfo?.title || '视频标题' }}
					</h1>
					<div class="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
						<span class="flex items-center gap-1.5">
							<UiIcon name="play" :size="14" />
							{{ formatCount(stats.play_count) }} 播放
						</span>
						<span class="flex items-center gap-1.5">
							<UiIcon name="thumbs-up" :size="14" />
							{{ formatCount(stats.like_count) }} 点赞
						</span>
						<span class="flex items-center gap-1.5">
							<UiIcon name="bookmark" :size="14" />
							{{ formatCount(stats.favorite_count) }} 收藏
						</span>
					</div>
					<div class="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground">
						<span v-if="formattedDate" class="flex items-center gap-1.5">
							<UiIcon name="clock" :size="14" /> 发布于 {{ formattedDate }}
						</span>
						<div class="flex items-center gap-1.5">
							<button
								class="flex items-center gap-1.5 rounded-full border border-border px-3 py-1.5 transition-colors hover:bg-accent"
								:class="hasLiked ? 'border-primary/40 bg-primary/10 text-primary' : 'text-muted-foreground hover:text-foreground'"
								type="button"
								@click="toggleLike"
							>
								<UiIcon name="thumbs-up" :size="15" :class="hasLiked ? 'fill-current' : ''" />
								<span>点赞</span>
							</button>
							<button
								class="flex items-center gap-1.5 rounded-full border border-border px-3 py-1.5 transition-colors hover:bg-accent"
								:class="hasFavorited ? 'border-primary/40 bg-primary/10 text-primary' : 'text-muted-foreground hover:text-foreground'"
								type="button"
								@click="toggleFavorite"
							>
								<UiIcon name="bookmark" :size="15" :class="hasFavorited ? 'fill-current' : ''" />
								<span>收藏</span>
							</button>
							<button
								class="flex items-center gap-1.5 rounded-full border border-border px-3 py-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
								type="button"
								@click="handleForward"
							>
								<UiIcon name="share" :size="15" />
								<span>转发</span>
							</button>
						</div>
					</div>
					<div class="border-t border-border pt-3 text-sm leading-relaxed text-foreground/90">
						{{ videoInfo?.description || '暂无简介' }}
					</div>
				</div>
			</div>

			<!-- Sidebar: Author + Rec -->
			<div class="w-full space-y-4 lg:w-80 lg:shrink-0">
				<div class="glass rounded-2xl p-4">
					<div class="flex items-center gap-3">
						<div
							class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] font-semibold text-white ring-2 ring-white/10"
						>
							<img
								v-if="videoInfo?.user?.avatar_url"
								:src="videoInfo.user.avatar_url"
								alt="avatar"
								class="h-full w-full object-cover"
							/>
							<span v-else>{{ (videoInfo?.user?.nickname || 'V')[0] }}</span>
						</div>
						<div class="min-w-0">
							<div class="truncate font-semibold text-foreground">
								{{ videoInfo?.user?.nickname || '未知UP主' }}
							</div>
							<div class="truncate text-xs text-muted-foreground">这家伙很懒，什么都没写</div>
						</div>
					</div>
					<div class="mt-4 flex gap-2">
						<button
							class="flex-1 rounded-lg bg-gradient-to-r from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] py-2 text-sm font-medium text-white shadow-lg shadow-black/20 transition-all hover:brightness-110 active:scale-[0.98]"
						>
							+ 关注
						</button>
						<button class="glass flex-1 rounded-lg py-2 text-sm text-foreground transition-colors hover:bg-accent">
							私信
						</button>
					</div>
				</div>

				<!-- 相关推荐 -->
				<div class="glass rounded-2xl p-4">
					<h3 class="mb-3 flex items-center gap-1.5 font-semibold text-foreground">
						<UiIcon name="trending" :size="16" class="text-primary" /> 相关推荐
					</h3>
					<div v-if="relatedLoading" class="space-y-3">
						<div v-for="i in 5" :key="i" class="flex gap-2.5">
							<div class="h-16 w-28 flex-shrink-0 animate-pulse rounded-lg bg-secondary"></div>
							<div class="flex flex-1 flex-col justify-between py-0.5">
								<div class="h-3.5 w-full animate-pulse rounded bg-secondary"></div>
								<div class="h-3 w-1/2 animate-pulse rounded bg-secondary"></div>
							</div>
						</div>
					</div>
					<div v-else-if="relatedVideos.length > 0" class="space-y-2">
						<div
							v-for="item in relatedVideos"
							:key="item.id"
							class="group flex cursor-pointer gap-2.5 rounded-lg p-1.5 transition-colors hover:bg-accent"
							@click="goVideo(item.id)"
						>
							<div class="relative h-16 w-28 flex-shrink-0 overflow-hidden rounded-lg bg-secondary">
								<img
									v-if="item.cover_url"
									:src="item.cover_url"
									alt="cover"
									class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
									loading="lazy"
								/>
								<div
									class="absolute bottom-1 right-1 rounded bg-black/70 px-1 py-0.5 text-[10px] tabular-nums text-white"
								>
									{{ formatDuration(item.duration) }}
								</div>
							</div>
							<div class="flex min-w-0 flex-1 flex-col justify-between py-0.5">
								<div class="line-clamp-2 text-sm font-medium leading-snug text-foreground transition-colors group-hover:text-primary">
									{{ item.title }}
								</div>
								<div class="flex items-center gap-1 text-xs text-muted-foreground">
									<span class="truncate">{{ item.user?.nickname || '未知UP主' }}</span>
									<span v-if="item.created_at" class="text-muted-foreground/50">·</span>
									<span v-if="item.created_at" class="shrink-0">{{ formatRelativeDate(item.created_at) }}</span>
								</div>
							</div>
						</div>
					</div>
					<div v-else class="py-6 text-center text-xs text-muted-foreground">暂无相关推荐</div>
				</div>
			</div>
		</div>
	</BiliLayout>
</template>
