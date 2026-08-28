<script setup lang="ts">
import { computed, ref } from 'vue'

export interface QualityOption {
	index: number
	height: number
	bandwidth: number
	label: string
}

const props = defineProps<{
	options: QualityOption[]
	value: 'auto' | number
}>()

const emit = defineEmits<{
	(e: 'change', value: 'auto' | number): void
}>()

const isOpen = ref(false)

const currentLabel = computed(() => {
	if (props.value === 'auto') return '自动'
	const found = props.options.find(o => o.index === props.value)
	return found?.label ?? '自动'
})

function handleSelect(val: 'auto' | number) {
	emit('change', val)
	isOpen.value = false
}
</script>

<template>
	<div class="relative flex items-center text-xs text-white">
		<button
			type="button"
			class="flex items-center rounded bg-white/10 px-2 py-0.5 transition-colors hover:bg-white/25"
			@click="isOpen = !isOpen"
		>
			<span class="mr-1 hidden sm:inline">清晰度</span>
			<span>{{ currentLabel }}</span>
		</button>
		<Transition name="menu">
			<div v-if="isOpen" class="absolute bottom-8 right-0 z-20 w-24 rounded-lg bg-black/90 py-1 text-xs shadow-xl ring-1 ring-white/10">
				<button
					type="button"
					class="block w-full px-3 py-1.5 text-left transition-colors"
					:class="props.value === 'auto' ? 'text-primary' : 'text-white/80 hover:text-white'"
					@click="handleSelect('auto')"
				>
					自动
				</button>
				<button
					v-for="q in props.options"
					:key="q.index"
					type="button"
					class="block w-full px-3 py-1.5 text-left transition-colors"
					:class="props.value === q.index ? 'text-primary' : 'text-white/80 hover:text-white'"
					@click="handleSelect(q.index)"
				>
					{{ q.label }}
				</button>
			</div>
		</Transition>
	</div>
</template>

<style scoped>
.menu-enter-active,
.menu-leave-active {
	transition: opacity 0.15s ease, transform 0.15s ease;
}
.menu-enter-from,
.menu-leave-to {
	opacity: 0;
	transform: translateY(4px);
}
</style>
