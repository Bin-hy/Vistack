<script setup lang="ts">
import { UiButton } from '@/components/ui'
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
	<div class="min-h-screen bg-[hsl(var(--background))] text-[hsl(var(--foreground))]">
		<header class="border-b border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80">
			<div class="max-w-[1200px] mx-auto px-4 h-14 flex items-center justify-between gap-4">
				<div class="flex items-center gap-2">
					<img src="/logo.png" alt="logo" class="h-8 w-8 rounded" />
					<span class="font-bold text-blue-600">Vistack Admin</span>
				</div>
				<div class="flex items-center gap-3">
					<UiButton variant="link" size="sm" class="!text-blue-600 !no-underline" @click="router.push('/login')">登录</UiButton>
					<UiButton variant="link" size="sm" class="!text-blue-600 !no-underline" @click="router.push('/register')">注册</UiButton>
				</div>
			</div>
		</header>
		<main class="max-w-[1200px] mx-auto px-4 py-6">
			<slot />
		</main>
		<footer class="py-6 text-center text-xs text-gray-500">© Vistack</footer>
	</div>
</template>

<style scoped>
</style>
