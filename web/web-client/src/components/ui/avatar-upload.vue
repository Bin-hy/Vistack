<script setup lang="ts">
import { ref, watch } from 'vue'

const props = withDefaults(
	defineProps<{
		previewUrl?: string
		class?: string
	}>(),
	{
		previewUrl: '',
		class: '',
	},
)

const emit = defineEmits<{
	(e: 'change', file: File | null): void
}>()

const internalPreview = ref<string | null>(props.previewUrl || null)

watch(
	() => props.previewUrl,
	(url) => {
		internalPreview.value = url || null
	},
)

function onFileChange(e: Event) {
	const target = e.target as HTMLInputElement
	const file = target.files && target.files[0]
	if (!file) {
		internalPreview.value = props.previewUrl || null
		emit('change', null)
		return
	}
	emit('change', file)
	const reader = new FileReader()
	reader.onload = () => {
		internalPreview.value = typeof reader.result === 'string' ? reader.result : null
	}
	reader.readAsDataURL(file)
}
</script>

<template>
	<div class="flex items-center gap-4" :class="props.class">
		<div
			class="h-16 w-16 rounded-full bg-gray-100 border border-[hsl(var(--border))] overflow-hidden flex items-center justify-center"
		>
			<img
				v-if="internalPreview"
				:src="internalPreview"
				alt="avatar"
				class="h-full w-full object-cover"
			/>
			<span v-else class="text-[10px] text-gray-400">暂无头像</span>
		</div>
		<div class="flex flex-col gap-1 text-xs text-gray-500">
			<label
				class="inline-flex items-center justify-center h-9 px-3 rounded-full bg-[hsl(var(--primary))] text-white text-xs cursor-pointer hover:bg-[hsl(var(--primary)/0.9)]"
			>
				<span>更换头像</span>
				<input type="file" accept="image/*" class="hidden" @change="onFileChange" />
			</label>
			<span>支持 JPG/PNG 等图片格式，最大 5MB</span>
		</div>
	</div>
</template>

