import { ref, onMounted, onBeforeUnmount, watch, type Ref } from 'vue'
import * as dashjs from 'dashjs'
import type { VideoSegmentsSignatureCredentials } from '@/views/VideoPlayer'
import { signS3Request, type AwsCredentials } from '@/lib/s3-signer'

export interface DashPlayerOptions {
	src: Ref<string>
	autoplay: Ref<boolean>
	segmentsBaseUrl: Ref<string>
	segmentsCredentials: Ref<VideoSegmentsSignatureCredentials | null>
}

export interface DashQualityOption {
	index: number
	height: number
	bandwidth: number
	label: string
}

export function useDashPlayer(options: DashPlayerOptions) {
	const videoRef = ref<HTMLVideoElement | null>(null)
	let player: dashjs.MediaPlayerClass | null = null
	let currentBaseUrl = ''
	let currentCredentials: AwsCredentials | null = null
	const s3Region = 'us-east-1'
	const qualityOptions = ref<DashQualityOption[]>([])
	const selectedQuality = ref<'auto' | number>('auto')
	const isPlaying = ref(false)
	const currentTime = ref(0)
	const duration = ref(0)
	const volume = ref(1)
	const isMuted = ref(false)
	const isReady = ref(false)

	function updateStsConfig() {
		currentBaseUrl = options.segmentsBaseUrl.value || ''
		if (options.segmentsCredentials.value) {
			currentCredentials = {
				accessKey: options.segmentsCredentials.value.accessKey,
				secretKey: options.segmentsCredentials.value.secretKey,
				sessionToken: options.segmentsCredentials.value.sessionToken,
			}
		} else {
			currentCredentials = null
		}
	}

	function buildSegmentUrl(originalUrl: string): string {
		if (!currentBaseUrl) return originalUrl
		try {
			const u = new URL(originalUrl, window.location.href)
			const parts = u.pathname.split('/')
			const filename = parts[parts.length - 1] || ''
			if (!filename.endsWith('.m4s')) return originalUrl
			const base = currentBaseUrl.endsWith('/') ? currentBaseUrl : currentBaseUrl + '/'
			const finalUrl = base + filename
			return finalUrl
		} catch {
			return originalUrl
		}
	}

	function setupPlayer(url: string) {
		if (!videoRef.value) return
		updateStsConfig()
		if (!player) {
			player = dashjs.MediaPlayer().create()
			player.updateSettings({ streaming: { abr: { autoSwitchBitrate: { video: true } } } } as any)
			player.addRequestInterceptor((req: any) => {
				if (!currentBaseUrl || !currentCredentials) return req
				const originalUrl = req.url as string
				const finalUrl = buildSegmentUrl(originalUrl)
				if (finalUrl === originalUrl) return req
				const headers = signS3Request(finalUrl, 'GET', s3Region, currentCredentials as AwsCredentials, new Date())
				req.url = finalUrl
				req.headers = {
					...(req.headers || {}),
					...headers,
				}
				return req
			})
			player.on('streamInitialized', () => {
				const reps = player?.getRepresentationsByType('video') || []
				const list = reps.map((r) => ({ index: r.index, height: r.height, bandwidth: r.bandwidth, label: `${r.height}p` }))
				list.sort((a, b) => b.height - a.height || b.bandwidth - a.bandwidth)
				qualityOptions.value = list
				const cur = player?.getCurrentRepresentationForType('video')
				selectedQuality.value = cur ? cur.index : 'auto'
				if (videoRef.value) {
					isReady.value = true
					currentTime.value = videoRef.value.currentTime || 0
					duration.value = videoRef.value.duration || 0
					volume.value = videoRef.value.volume
					isMuted.value = videoRef.value.muted
				}
			})
			player.initialize(videoRef.value, url, options.autoplay.value)
		} else {
			player.attachSource(url)
		}
	}

	function setQuality(q: 'auto' | number) {
		if (!player) return
		if (q === 'auto') {
			player.updateSettings({ streaming: { abr: { autoSwitchBitrate: { video: true } } } } as any)
			selectedQuality.value = 'auto'
			return
		}
		player.updateSettings({ streaming: { abr: { autoSwitchBitrate: { video: false } } } } as any)
		player.setRepresentationForTypeByIndex('video', q as number, true)
		selectedQuality.value = q
	}

	function togglePlay() {
		if (!videoRef.value) return
		if (videoRef.value.paused) {
			videoRef.value.play()
		} else {
			videoRef.value.pause()
		}
	}

	function seek(time: number) {
		if (!videoRef.value) return
		videoRef.value.currentTime = time
	}

	function changeVolume(v: number) {
		if (!videoRef.value) return
		videoRef.value.volume = v
		volume.value = v
		isMuted.value = videoRef.value.muted || v === 0
	}

	function toggleMute() {
		if (!videoRef.value) return
		videoRef.value.muted = !videoRef.value.muted
		isMuted.value = videoRef.value.muted
		volume.value = videoRef.value.volume
	}

	onMounted(() => {
		if (options.src.value) {
			setupPlayer(options.src.value)
		}
		const v = videoRef.value
		if (v) {
			v.addEventListener('timeupdate', () => {
				currentTime.value = v.currentTime || 0
				duration.value = v.duration || 0
			})
			v.addEventListener('play', () => {
				isPlaying.value = true
			})
			v.addEventListener('pause', () => {
				isPlaying.value = false
			})
			v.addEventListener('volumechange', () => {
				volume.value = v.volume
				isMuted.value = v.muted || v.volume === 0
			})
		}
	})

	watch(
		() => options.src.value,
		(val) => {
			if (val) {
				setupPlayer(val)
			}
		},
	)

	watch(
		() => options.segmentsCredentials.value,
		() => {
			updateStsConfig()
		},
	)

	watch(
		() => options.segmentsBaseUrl.value,
		() => {
			updateStsConfig()
		},
	)

	onBeforeUnmount(() => {
		if (player) {
			player.reset()
			player = null
		}
	})

	return {
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
	}
}
