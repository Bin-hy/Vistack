<script setup lang="ts">
const props = withDefaults(
	defineProps<{
		modelValue?: string
		placeholder?: string
		type?: string
		class?: string
	}>(),
	{
		modelValue: '',
		placeholder: '',
		type: 'text',
		class: '',
	},
)

const emit = defineEmits<{
	(e: 'update:modelValue', value: string): void
}>()

function onInput(e: Event) {
	const target = e.target as HTMLInputElement
	emit('update:modelValue', target.value)
}
</script>

<template>
	<input
		:placeholder="props.placeholder"
		:type="props.type"
		:value="props.modelValue"
		class="
			flex h-11 w-full rounded-md border border-border bg-input/80 px-3 py-2 text-sm text-foreground
			placeholder:text-muted-foreground
			transition-all duration-200
			focus-visible:border-primary/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:bg-input
			hover:border-border/80
			[&:disabled]:cursor-not-allowed [&:disabled]:opacity-60
			" :class="props.class"
		@input="onInput"
	/>
</template>
