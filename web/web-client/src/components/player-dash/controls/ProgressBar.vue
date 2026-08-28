<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps<{
	currentTime: number
	duration: number
}>()

const emit = defineEmits<{
	(e: 'update:time', value: number): void
}>()

const barRef = ref<HTMLElement | null>(null)
const dragging = ref(false)

const pct = computed(() => {
	if (!props.duration) return 0
	return Math.min(100, Math.max(0, (props.currentTime / props.duration) * 100))
})

function ratioFromEvent(e: MouseEvent) {
	const bar = barRef.value
	if (!bar) return 0
	const rect = bar.getBoundingClientRect()
	const x = Math.min(rect.width, Math.max(0, e.clientX - rect.left))
	return x / rect.width
}

function seekTo(e: MouseEvent) {
	if (!props.duration) return
	emit('update:time', ratioFromEvent(e) * props.duration)
}

function onMouseDown(e: MouseEvent) {
	dragging.value = true
	seekTo(e)
	document.addEventListener('mousemove', onMouseMove)
	document.addEventListener('mouseup', onMouseUp)
}

function onMouseMove(e: MouseEvent) {
	if (dragging.value) seekTo(e)
}

function onMouseUp() {
	dragging.value = false
	document.removeEventListener('mousemove', onMouseMove)
	document.removeEventListener('mouseup', onMouseUp)
}
</script>

<template>
	<div
		ref="barRef"
		class="group relative flex h-4 w-full cursor-pointer items-center"
		@mousedown="onMouseDown"
	>
		<div class="relative h-1 w-full rounded-full bg-white/20 transition-all duration-150 group-hover:h-1.5">
			<div
				class="absolute inset-y-0 left-0 rounded-full bg-gradient-to-r from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))]"
				:style="{ width: pct + '%' }"
			></div>
			<div
				class="absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white shadow-md transition-opacity"
				:class="dragging ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'"
				:style="{ left: pct + '%' }"
			></div>
		</div>
	</div>
</template>
