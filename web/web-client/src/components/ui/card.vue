<script setup lang="ts">
const props = defineProps<{ class?: string }>()
</script>

<template>
	<div
		:class="[
			'bg-white/95 dark:bg-neutral-900/90 border border-[hsl(var(--border))] rounded-xl shadow-sm',
			'overflow-hidden',
			props.class,
		]"
	>
		<slot />
	</div>
</template>
