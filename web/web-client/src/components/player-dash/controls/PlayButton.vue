<script setup lang="ts">
const props = defineProps<{
	playing: boolean
}>()

const emit = defineEmits<{
	(e: 'toggle'): void
}>()

function handleClick() {
	emit('toggle')
}
</script>

<template>
	<button
		class="flex h-8 w-8 items-center justify-center rounded-full bg-white/10 text-white hover:bg-white/20"
		type="button"
		@click="handleClick"
	>
		<span v-if="props.playing" class="text-xs">⏸</span>
		<span v-else class="text-xs">▶</span>
	</button>
</template>
