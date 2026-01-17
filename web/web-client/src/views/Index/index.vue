<script setup lang="ts">
import BiliLayout from '@/layouts/BiliLayout.vue'
import { useRouter } from 'vue-router'
import { UiButton } from '@/components/ui'

const router = useRouter()

function goCreatorCenter() {
	router.push('/creator')
}
</script>

<template>
	<BiliLayout>
		<div class="flex items-start justify-between gap-6">
			<div
				class="flex-1 rounded-lg border border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80 p-6"
			>
				<h1 class="text-xl font-semibold">首页</h1>
				<p class="mt-2 text-sm text-gray-500">这里将展示推荐内容。</p>
			</div>
			<div class="w-56 flex flex-col items-stretch gap-3">
				<UiButton
					variant="default"
					size="lg"
					class="w-full bg-pink-500 hover:bg-pink-600 text-white shadow-md"
					@click="goCreatorCenter"
				>
					投稿
				</UiButton>
				<p class="text-xs text-gray-500 text-center">前往创作中心，上传你的视频</p>
			</div>
		</div>
	</BiliLayout>
</template>

<style scoped>
</style>
