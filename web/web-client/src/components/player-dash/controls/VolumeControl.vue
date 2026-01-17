<script setup lang="ts">
const props = defineProps<{
	volume: number
	muted: boolean
}>()

const emit = defineEmits<{
	(e: 'update:volume', value: number): void
	(e: 'toggle-mute'): void
}>()

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
	<div class="flex items-center gap-1">
		<button
			class="flex h-8 w-8 items-center justify-center rounded-full bg-white/10 text-white hover:bg-white/20"
			type="button"
			@click="handleMuteClick"
		>
			<span v-if="props.muted || props.volume === 0" class="text-xs">🔇</span>
			<span v-else-if="props.volume < 0.5" class="text-xs">🔈</span>
			<span v-else class="text-xs">🔊</span>
		</button>
		<input
			class="w-20 cursor-pointer accent-[#00A1D6]"
			type="range"
			min="0"
			max="1"
			step="0.05"
			:value="props.muted ? 0 : props.volume"
			@input="handleVolumeInput"
		/>
	</div>
</template>
