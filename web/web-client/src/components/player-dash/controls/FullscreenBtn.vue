<script setup lang="ts">
import { UiIcon } from '@/components/ui'

const props = defineProps<{
	fullscreen: boolean
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
		class="flex h-8 w-8 items-center justify-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/25"
		type="button"
		@click="handleClick"
	>
		<UiIcon :name="props.fullscreen ? 'minimize' : 'maximize'" :size="16" />
	</button>
</template>
