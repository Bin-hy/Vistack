<script setup lang="ts">
import { computed } from 'vue'
import { UiIcon } from '@/components/ui'

const props = defineProps<{
	volume: number
	muted: boolean
}>()

const emit = defineEmits<{
	(e: 'update:volume', value: number): void
	(e: 'toggle-mute'): void
}>()

const icon = computed(() => {
	if (props.muted || props.volume === 0) return 'volume-x'
	if (props.volume < 0.5) return 'volume-low'
	return 'volume'
})

function handleVolumeInput(event: Event) {
	const target = event.target as HTMLInputElement
	const value = Number(target.value)
	emit('update:volume', value)
}

function handleMuteClick() {
	emit('toggle-mute')
}
</script>

<template>
	<div class="flex items-center gap-1.5">
		<button
			class="flex h-8 w-8 items-center justify-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/25"
			type="button"
			@click="handleMuteClick"
		>
			<UiIcon :name="icon" :size="16" />
		</button>
		<input
			class="w-20 cursor-pointer"
			type="range"
			min="0"
			max="1"
			step="0.05"
			:value="props.muted ? 0 : props.volume"
			@input="handleVolumeInput"
		/>
	</div>
</template>
