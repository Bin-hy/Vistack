<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import * as dashjs from 'dashjs'
import type { VideoSegmentsSignatureCredentials } from '@/views/VideoPlayer'
import { signS3Request, type AwsCredentials } from '@/lib/s3-signer'

const props = withDefaults(defineProps<{
	src: string
	autoplay?: boolean
	controls?: boolean
	muted?: boolean
	preload?: 'auto' | 'metadata' | 'none'
	segmentsBaseUrl?: string
	segmentsCredentials?: VideoSegmentsSignatureCredentials | null
}>(), {
	autoplay: false,
	controls: true,
	muted: false,
	preload: 'metadata',
	segmentsBaseUrl: '',
	segmentsCredentials: null,
})

const videoRef = ref<HTMLVideoElement | null>(null)
let player: dashjs.MediaPlayerClass | null = null
let currentBaseUrl = ''
let currentCredentials: AwsCredentials | null = null
const s3Region = 'us-east-1'
const qualityOptions = ref<{ index: number; height: number; bandwidth: number; label: string }[]>([])
const selectedQuality = ref<'auto' | number>('auto')

function updateStsConfig() {
	currentBaseUrl = props.segmentsBaseUrl || ''
	if (props.segmentsCredentials) {
		currentCredentials = {
			accessKey: props.segmentsCredentials.accessKey,
			secretKey: props.segmentsCredentials.secretKey,
			sessionToken: props.segmentsCredentials.sessionToken,
		}
	} else {
		currentCredentials = null
	}
	console.log('[dash] updateStsConfig', {
		segmentsBaseUrl: props.segmentsBaseUrl,
		hasCredentials: !!props.segmentsCredentials,
	})
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
		console.log('[dash] buildSegmentUrl', { originalUrl, finalUrl, currentBaseUrl })
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
        })
        player.initialize(videoRef.value, url, props.autoplay)
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

onMounted(() => {
	if (props.src) setupPlayer(props.src)
})

watch(
	() => props.src,
	(val) => {
		if (val) setupPlayer(val)
	},
)

watch(
	() => props.segmentsCredentials,
	() => {
		updateStsConfig()
	},
)

watch(
	() => props.segmentsBaseUrl,
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
</script>

<template>
    <div class="w-full flex justify-center">
        <video
            ref="videoRef"
      :controls="props.controls"
      :autoplay="props.autoplay"
      :muted="props.muted"
      :preload="props.preload"
      class="w-full max-w-4xl bg-black rounded-lg"
    />
  </div>
  <div class="w-full max-w-4xl mx-auto mt-2 flex items-center gap-2">
    <label class="text-sm text-foreground">清晰度</label>
    <select class="rounded border border-border bg-input px-2 py-1 text-sm text-foreground" :value="selectedQuality" @change="setQuality(($event.target as HTMLSelectElement).value === 'auto' ? 'auto' : Number(($event.target as HTMLSelectElement).value))">
      <option value="auto">自动</option>
      <option v-for="q in qualityOptions" :key="q.index" :value="q.index">{{ q.label }}（{{ (q.bandwidth/1000000).toFixed(2) }} Mbps）</option>
    </select>
  </div>
</template>
