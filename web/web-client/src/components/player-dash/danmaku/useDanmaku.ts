import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import type { DanmakuItem, DanmakuMode, DanmakuSettings } from './types'

export interface DanmakuSendPayload {
	content: string
	timeOffset: number
	mode: number
	color: string
}

interface PendingItem {
	text: string
	color: string
	mode: number
}

interface ActiveItem extends PendingItem {
	id: number
	track: number
	spawnTime: number
	width: number
}

const FONT_SCALE: Record<DanmakuSettings['fontSize'], number> = {
	small: 0.8,
	normal: 1,
	large: 1.25,
}

const AREA_FRACTION: Record<number, number> = { 1: 0.25, 2: 0.5, 3: 0.75, 4: 1 }

const FIXED_DURATION = 5 // 顶部/底部固定弹幕停留秒数
const SCROLL_GAP = 32 // 滚动弹幕首尾间距(px)
const PENDING_LIMIT = 400

const FONT_STACK = "'Inter','PingFang SC','HarmonyOS Sans SC','Microsoft YaHei',sans-serif"

export function useDanmaku(opts: {
	video: Ref<HTMLVideoElement | null>
	layer: Ref<HTMLElement | null>
	items: Ref<DanmakuItem[]>
	onSend: (payload: DanmakuSendPayload) => void
}) {
	const canvasRef = ref<HTMLCanvasElement | null>(null)

	const settings = ref<DanmakuSettings>({
		enabled: true,
		opacity: 1,
		area: 4,
		fontSize: 'normal',
		speed: 8,
	})

	const inputText = ref('')
	const inputMode = ref<DanmakuMode>(0)
	const inputColor = ref('#FFFFFF')
	const sentCount = ref(0)

	let ctx: CanvasRenderingContext2D | null = null
	let containerWidth = 0
	let containerHeight = 0

	let active: ActiveItem[] = []
	let pending: PendingItem[] = []
	const scrollTracks: number[] = []
	const topTracks: number[] = []
	const bottomTracks: number[] = []

	let nextId = 1
	let cursor = 0
	let displayedTime = 0
	let lastTs = 0
	let rafId = 0
	let resizeObserver: ResizeObserver | null = null

	const sortedItems = computed(() => [...opts.items.value].sort((a, b) => a.time_offset - b.time_offset))
	const count = computed(() => opts.items.value.length + sentCount.value)

	function fontSizePx(): number {
		return Math.max(14, containerWidth * 0.026) * FONT_SCALE[settings.value.fontSize]
	}

	function lineHeightPx(): number {
		return fontSizePx() + 12
	}

	function areaFraction(): number {
		return AREA_FRACTION[settings.value.area] ?? 1
	}

	function laneCount(): number {
		return Math.max(1, Math.floor((containerHeight * areaFraction()) / lineHeightPx()))
	}

	function speedPxPerSec(): number {
		return containerWidth / Math.max(2, settings.value.speed)
	}

	function fontString(): string {
		return `500 ${fontSizePx()}px ${FONT_STACK}`
	}

	function measure(text: string): number {
		if (ctx) {
			ctx.font = fontString()
			return ctx.measureText(text).width
		}
		const f = fontSizePx()
		let w = 0
		for (const ch of text) {
			const c = ch.codePointAt(0) || 0
			w += c > 0x2e80 ? f : f * 0.55
		}
		return Math.ceil(w)
	}

	function resetTracks() {
		const lanes = laneCount()
		scrollTracks.length = 0
		topTracks.length = 0
		bottomTracks.length = 0
		for (let i = 0; i < lanes; i++) {
			scrollTracks.push(-Infinity)
			topTracks.push(-Infinity)
			bottomTracks.push(-Infinity)
		}
	}

	function findTrack(arr: number[], now: number): number {
		for (let i = 0; i < arr.length; i++) {
			if ((arr[i] ?? -Infinity) <= now) return i
		}
		return -1
	}

	function spawn(d: PendingItem, now: number): boolean {
		const mode: number = d.mode === 1 || d.mode === 2 ? d.mode : 0
		const width = measure(d.text)

		if (mode === 1 || mode === 2) {
			const arr = mode === 1 ? topTracks : bottomTracks
			const track = findTrack(arr, now)
			if (track < 0) return false
			arr[track] = now + FIXED_DURATION
			active.push({ ...d, id: nextId++, track, spawnTime: now, width })
			return true
		}

		const track = findTrack(scrollTracks, now)
		if (track < 0) return false
		scrollTracks[track] = now + (width + SCROLL_GAP) / speedPxPerSec()
		active.push({ ...d, id: nextId++, track, spawnTime: now, width })
		return true
	}

	function spawnDue(now: number) {
		const list = sortedItems.value
		while (cursor < list.length) {
			const d = list[cursor]
			if (!d || d.time_offset > now) break
			const item: PendingItem = { text: d.content, color: d.color || '#FFFFFF', mode: d.mode ?? 0 }
			if (!spawn(item, now)) pending.push(item)
			cursor++
		}
	}

	function drainPending(now: number) {
		if (pending.length === 0) return
		const rest: PendingItem[] = []
		for (const d of pending) {
			if (!spawn(d, now)) rest.push(d)
		}
		pending = rest
		if (pending.length > PENDING_LIMIT) {
			pending.splice(0, pending.length - PENDING_LIMIT)
		}
	}

	function resetCursorTo(now: number) {
		const list = sortedItems.value
		let lo = 0
		let hi = list.length
		while (lo < hi) {
			const mid = (lo + hi) >> 1
			if ((list[mid]?.time_offset ?? 0) <= now) lo = mid + 1
			else hi = mid
		}
		cursor = lo
	}

	function render(now: number) {
		if (!ctx || !canvasRef.value) return
		ctx.clearRect(0, 0, containerWidth, containerHeight)
		if (!settings.value.enabled) return

		const lh = lineHeightPx()
		const speed = speedPxPerSec()
		ctx.font = fontString()
		ctx.textBaseline = 'middle'
		ctx.shadowColor = 'rgba(0,0,0,0.85)'
		ctx.shadowBlur = 2

		const keep: ActiveItem[] = []

		for (const item of active) {
			let alpha = settings.value.opacity

			if (item.mode === 1 || item.mode === 2) {
				const elapsed = now - item.spawnTime
				if (elapsed >= FIXED_DURATION) continue
				const fadeIn = Math.min(1, elapsed / 0.3)
				const fadeOut = Math.min(1, (FIXED_DURATION - elapsed) / 0.5)
				alpha *= Math.max(0, Math.min(1, Math.min(fadeIn, fadeOut)))
				const y = item.mode === 1 ? item.track * lh + lh / 2 : containerHeight - item.track * lh - lh / 2
				ctx.fillStyle = item.color
				ctx.globalAlpha = alpha
				ctx.textAlign = 'center'
				ctx.fillText(item.text, containerWidth / 2, y)
				keep.push(item)
				continue
			}

			const elapsed = now - item.spawnTime
			const x = containerWidth - elapsed * speed
			if (x + item.width < -24) continue
			ctx.fillStyle = item.color
			ctx.globalAlpha = alpha
			ctx.textAlign = 'left'
			ctx.fillText(item.text, x, item.track * lh + lh / 2)
			keep.push(item)
		}

		ctx.globalAlpha = 1
		ctx.shadowBlur = 0
		active = keep
	}

	function frame(ts: number) {
		rafId = requestAnimationFrame(frame)
		const video = opts.video.value
		if (!video || !ctx) return

		const dt = lastTs ? (ts - lastTs) / 1000 : 0
		lastTs = ts

		if (!video.paused) {
			displayedTime += dt * video.playbackRate
		} else {
			displayedTime = video.currentTime
		}

		// 跳转/seek 检测与重同步
		if (Math.abs(displayedTime - video.currentTime) > 1) {
			displayedTime = video.currentTime
			active = []
			pending = []
			resetTracks()
			resetCursorTo(displayedTime)
		}

		spawnDue(displayedTime)
		drainPending(displayedTime)
		render(displayedTime)
	}

	function resize() {
		const layer = opts.layer.value
		const canvas = canvasRef.value
		if (!layer || !canvas) return
		const rect = layer.getBoundingClientRect()
		const dpr = window.devicePixelRatio || 1
		canvas.width = Math.max(1, Math.floor(rect.width * dpr))
		canvas.height = Math.max(1, Math.floor(rect.height * dpr))
		canvas.style.width = rect.width + 'px'
		canvas.style.height = rect.height + 'px'
		ctx = canvas.getContext('2d')
		if (ctx) ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
		containerWidth = rect.width
		containerHeight = rect.height
		resetTracks()
	}

	function setMode(mode: DanmakuMode) {
		inputMode.value = mode
	}

	function setColor(color: string) {
		inputColor.value = color
	}

	function toggleEnabled() {
		settings.value.enabled = !settings.value.enabled
	}

	function send() {
		const text = inputText.value.trim()
		if (!text) return
		const video = opts.video.value
		const time = video ? video.currentTime : displayedTime
		const item: PendingItem = { text, color: inputColor.value, mode: inputMode.value }
		if (!spawn(item, time)) pending.push(item)
		inputText.value = ''
		sentCount.value++
		opts.onSend({ content: text, timeOffset: time, mode: inputMode.value, color: inputColor.value })
	}

	onMounted(() => {
		resize()
		if (opts.layer.value) {
			resizeObserver = new ResizeObserver(() => resize())
			resizeObserver.observe(opts.layer.value)
		}
		lastTs = 0
		rafId = requestAnimationFrame(frame)
	})

	onBeforeUnmount(() => {
		if (rafId) cancelAnimationFrame(rafId)
		if (resizeObserver) {
			resizeObserver.disconnect()
			resizeObserver = null
		}
	})

	watch(
		() => opts.items.value,
		() => {
			cursor = 0
			active = []
			pending = []
			resetTracks()
		},
	)

	watch([() => settings.value.fontSize, () => settings.value.area], () => resetTracks())

	return {
		canvasRef,
		settings,
		count,
		inputText,
		inputMode,
		inputColor,
		setMode,
		setColor,
		toggleEnabled,
		send,
	}
}

export type DanmakuEngine = ReturnType<typeof useDanmaku>
