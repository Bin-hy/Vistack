<script setup lang="ts">
const props = defineProps<{ class?: string }>()
</script>

<template>
	<div
		:class="[
			'glass rounded-xl shadow-lg shadow-black/10 overflow-hidden',
			props.class,
		]"
	>
		<slot />
	</div>
</template>
