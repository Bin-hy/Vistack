<script setup lang="ts">
const props = defineProps<{ class?: string }>()
</script>

<template>
	<div
		:class="[
			'glass rounded-xl shadow-soft overflow-hidden',
			props.class,
		]"
	>
		<slot />
	</div>
</template>
