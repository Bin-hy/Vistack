<script setup lang="ts">
const props = defineProps<{
	currentTime: number
	duration: number
}>()

const emit = defineEmits<{
	(e: 'update:time', value: number): void
}>()

function handleInput(event: Event) {
	const target = event.target as HTMLInputElement
	const value = Number(target.value)
	emit('update:time', value)
}
</script>

<template>
	<div class="flex w-full items-center">
		<input
			class="w-full cursor-pointer accent-[#00A1D6]"
			type="range"
			min="0"
			:max="props.duration || 0"
			step="0.1"
			:value="props.currentTime"
			@input="handleInput"
		/>
	</div>
</template>
