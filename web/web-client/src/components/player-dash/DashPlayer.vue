<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, toRef, watch } from 'vue'
import type { VideoSegmentsSignatureCredentials } from '@/views/VideoPlayer'
import PlayButton from './controls/PlayButton.vue'
import ProgressBar from './controls/ProgressBar.vue'
import VolumeControl from './controls/VolumeControl.vue'
import FullscreenBtn from './controls/FullscreenBtn.vue'
import QualitySelector from './controls/QualitySelector.vue'
import DanmakuBar from './danmaku/DanmakuBar.vue'
import { useDanmaku } from './danmaku/useDanmaku'
import type { DanmakuItem } from './danmaku/types'
import { useDashPlayer } from './useDashPlayer'

const props = withDefaults(defineProps<{
	src: string
	autoplay?: boolean
	muted?: boolean
	preload?: 'auto' | 'metadata' | 'none'
	segmentsBaseUrl?: string
	segmentsCredentials?: VideoSegmentsSignatureCredentials | null
	danmakus?: DanmakuItem[]
}>(), {
	autoplay: false,
	muted: false,
	preload: 'metadata',
	segmentsBaseUrl: '',
	segmentsCredentials: null,
	danmakus: () => [],
})

const emit = defineEmits<{
	(e: 'send-danmaku', text: string, timeOffset: number, mode: number, color: string): void
}>()

const srcRef = ref(props.src)
const autoplayRef = ref(props.autoplay)
const segmentsBaseUrlRef = ref(props.segmentsBaseUrl || '')
const segmentsCredentialsRef = ref<VideoSegmentsSignatureCredentials | null>(props.segmentsCredentials)

const {
	videoRef,
	isReady,
	isPlaying,
	currentTime,
	duration,
	volume,
	isMuted,
	qualityOptions,
	selectedQuality,
	setQuality,
	togglePlay,
	seek,
	changeVolume,
	toggleMute,
} = useDashPlayer({
	src: srcRef,
	autoplay: autoplayRef,
	segmentsBaseUrl: segmentsBaseUrlRef,
	segmentsCredentials: segmentsCredentialsRef,
})

const containerRef = ref<HTMLDivElement | null>(null)
const isFullscreen = ref(false)

const danmakuLayerRef = ref<HTMLElement | null>(null)
const danmaku = useDanmaku({
	video: videoRef,
	layer: danmakuLayerRef,
	items: toRef(props, 'danmakus'),
	onSend: (p) => emit('send-danmaku', p.content, p.timeOffset, p.mode, p.color),
})

let clickTimer: number | null = null
let rightPressTimer: number | null = null
let rightKeyTimer: number | null = null
const speedHoldActive = ref(false)
let suppressNextContext = false

const playbackRate = ref(1)
const rateOptions = [0.5, 1, 1.25, 1.5, 2]
const showRateMenu = ref(false)
const displayPlaybackRate = computed(() => (speedHoldActive.value ? 1.5 : playbackRate.value))

function formatTime(value: number) {
	if (!Number.isFinite(value) || value <= 0) return '00:00'
	const total = Math.floor(value)
	const m = Math.floor(total / 60)
	const s = total % 60
	const mm = m.toString().padStart(2, '0')
	const ss = s.toString().padStart(2, '0')
	return `${mm}:${ss}`
}

const formattedCurrentTime = computed(() => formatTime(currentTime.value))
const formattedDuration = computed(() => formatTime(duration.value))

function toggleFullscreen() {
	const el = containerRef.value
	if (!el) return
	if (!document.fullscreenElement) {
		if (el.requestFullscreen) {
			el.requestFullscreen()
		}
	} else {
		if (document.exitFullscreen) {
			document.exitFullscreen()
		}
	}
}

function handleFullscreenChange() {
	isFullscreen.value = !!document.fullscreenElement
}

function handleVideoClick() {
	if (clickTimer !== null) return
	clickTimer = window.setTimeout(() => {
		clickTimer = null
		togglePlay()
	}, 220)
}

function handleVideoDblClick(event: MouseEvent) {
	event.preventDefault()
	if (clickTimer !== null) {
		clearTimeout(clickTimer)
		clickTimer = null
	}
	toggleFullscreen()
}

function setPlaybackRate(rate: number) {
	playbackRate.value = rate
	if (videoRef.value) {
		if (!speedHoldActive.value) {
			videoRef.value.playbackRate = rate
		}
	}
}

function toggleRateMenu() {
	showRateMenu.value = !showRateMenu.value
}

function handleSelectRate(rate: number) {
	setPlaybackRate(rate)
	showRateMenu.value = false
}

function handleVideoMouseDown(event: MouseEvent) {
	if (event.button !== 2) return
	event.preventDefault()
	if (rightPressTimer !== null) {
		clearTimeout(rightPressTimer)
	}
	rightPressTimer = window.setTimeout(() => {
		rightPressTimer = null
		speedHoldActive.value = true
		if (videoRef.value) {
			videoRef.value.playbackRate = 1.5
		}
		suppressNextContext = true
	}, 500)
}

function handleVideoMouseUp(event: MouseEvent) {
	if (event.button !== 2) return
	if (rightPressTimer !== null) {
		clearTimeout(rightPressTimer)
		rightPressTimer = null
	}
	if (speedHoldActive.value) {
		speedHoldActive.value = false
		if (videoRef.value) {
			videoRef.value.playbackRate = playbackRate.value
		}
	}
}

function handleVideoContextMenu(event: MouseEvent) {
	event.preventDefault()
	if (suppressNextContext) {
		suppressNextContext = false
		return
	}
	if (!videoRef.value) return
	const step = 5
	const target = videoRef.value.currentTime + step
	const maxDuration = duration.value || videoRef.value.duration || 0
	const next = maxDuration > 0 ? Math.min(target, maxDuration) : target
	videoRef.value.currentTime = next
}

function handleKeydown(event: KeyboardEvent) {
	const target = event.target as HTMLElement | null
	if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || (target as any).isContentEditable)) {
		return
	}
	if (!videoRef.value) return
	if (event.key === 'ArrowUp') {
		event.preventDefault()
		const step = 0.05
		const next = Math.min(1, (videoRef.value.volume || 0) + step)
		changeVolume(next)
	} else if (event.key === 'ArrowDown') {
		event.preventDefault()
		const step = 0.05
		const next = Math.max(0, (videoRef.value.volume || 0) - step)
		changeVolume(next)
	} else if (event.key === 'ArrowRight') {
		event.preventDefault()
		if (!event.repeat) {
			const step = 5
			const base = videoRef.value.currentTime || 0
			const maxDuration = duration.value || videoRef.value.duration || 0
			const next = maxDuration > 0 ? Math.min(base + step, maxDuration) : base + step
			seek(next)
			if (rightKeyTimer !== null) {
				clearTimeout(rightKeyTimer)
			}
			rightKeyTimer = window.setTimeout(() => {
				rightKeyTimer = null
				if (!speedHoldActive.value) {
					speedHoldActive.value = true
					if (videoRef.value) {
						videoRef.value.playbackRate = 1.5
					}
				}
			}, 500)
		}
	} else if (event.key === 'ArrowLeft') {
		event.preventDefault()
		if (!event.repeat) {
			const step = 5
			const base = videoRef.value.currentTime || 0
			const next = Math.max(0, base - step)
			seek(next)
		}
	}
}

function handleKeyup(event: KeyboardEvent) {
	if (event.key === 'ArrowRight') {
		if (rightKeyTimer !== null) {
			clearTimeout(rightKeyTimer)
			rightKeyTimer = null
		}
		if (speedHoldActive.value) {
			speedHoldActive.value = false
			if (videoRef.value) {
				videoRef.value.playbackRate = playbackRate.value
			}
		}
	}
}

onMounted(() => {
	window.addEventListener('keydown', handleKeydown)
	window.addEventListener('keyup', handleKeyup)
	if (videoRef.value) {
		videoRef.value.playbackRate = playbackRate.value
	}
	document.addEventListener('fullscreenchange', handleFullscreenChange)
})

onBeforeUnmount(() => {
	window.removeEventListener('keydown', handleKeydown)
	window.removeEventListener('keyup', handleKeyup)
	document.removeEventListener('fullscreenchange', handleFullscreenChange)
	if (clickTimer !== null) {
		clearTimeout(clickTimer)
		clickTimer = null
	}
	if (rightPressTimer !== null) {
		clearTimeout(rightPressTimer)
		rightPressTimer = null
	}
	if (rightKeyTimer !== null) {
		clearTimeout(rightKeyTimer)
		rightKeyTimer = null
	}
})

watch(
	() => props.src,
	val => {
		srcRef.value = val
	},
)

watch(
	() => props.autoplay,
	val => {
		autoplayRef.value = !!val
	},
)

watch(
	() => props.segmentsBaseUrl,
	val => {
		segmentsBaseUrlRef.value = val || ''
	},
)

watch(
	() => props.segmentsCredentials,
	val => {
		segmentsCredentialsRef.value = val || null
	},
)
</script>

<template>
	<div ref="containerRef" class="relative w-full max-w-5xl mx-auto overflow-hidden rounded-lg bg-black">
		<div class="relative w-full pb-[56.25%]">
			<video
				ref="videoRef"
				class="absolute inset-0 h-full w-full bg-black"
				:muted="muted"
				:autoplay="autoplay"
				:preload="preload"
				@click="handleVideoClick"
				@dblclick="handleVideoDblClick"
				@mousedown="handleVideoMouseDown"
				@mouseup="handleVideoMouseUp"
				@contextmenu="handleVideoContextMenu"
			></video>

			<!-- 弹幕层 -->
			<div ref="danmakuLayerRef" class="pointer-events-none absolute inset-0 overflow-hidden">
				<canvas :ref="danmaku.canvasRef" class="absolute inset-0 h-full w-full"></canvas>
			</div>

			<div v-if="isReady" class="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/80 via-black/10 to-transparent opacity-0 transition-opacity group-hover:opacity-100"></div>
		</div>
		<div class="absolute inset-x-0 bottom-0 z-10 bg-gradient-to-t from-black/80 via-black/40 to-transparent p-2 md:p-3">
			<div class="flex items-center gap-2 md:gap-3">
				<PlayButton :playing="isPlaying" @toggle="togglePlay" />
				<div class="flex flex-1 flex-col gap-1">
					<ProgressBar :current-time="currentTime" :duration="duration" @update:time="seek" />
					<div class="flex items-center justify-between text-xs text-white/80 scale-90 origin-left md:scale-100">
						<span>{{ formattedCurrentTime }} / {{ formattedDuration }}</span>
					</div>
				</div>
				<div class="hidden sm:block">
					<VolumeControl :volume="volume" :muted="isMuted" @update:volume="changeVolume" @toggle-mute="toggleMute" />
				</div>
				<div class="relative flex items-center text-xs text-white">
					<button
						type="button"
						class="flex items-center rounded bg-white/10 px-2 py-0.5 hover:bg-white/20"
						@click="toggleRateMenu"
					>
						<span class="mr-1 hidden sm:inline">倍速</span>
						<span>{{ displayPlaybackRate }}x</span>
					</button>
					<div
						v-if="showRateMenu"
						class="absolute bottom-8 right-0 z-20 w-20 rounded-lg bg-black/90 py-1 text-xs shadow-xl ring-1 ring-white/10"
					>
						<button
							v-for="rate in rateOptions"
							:key="rate"
							type="button"
							class="block w-full px-3 py-1 text-left"
							:class="rate === playbackRate ? 'text-primary' : 'text-white/80 hover:text-white'"
							@click="handleSelectRate(rate)"
						>
							{{ rate }}x
						</button>
					</div>
				</div>
				<QualitySelector :options="qualityOptions" :value="selectedQuality" @change="setQuality" />
				<FullscreenBtn :fullscreen="isFullscreen" @toggle="toggleFullscreen" />
			</div>

			<DanmakuBar :engine="danmaku" />
		</div>
	</div>
</template>
