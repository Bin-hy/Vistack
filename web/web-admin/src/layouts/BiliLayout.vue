<script setup lang="ts">
import { UiButton } from '@/components/ui'
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
	<div class="min-h-screen bg-[hsl(var(--background))] text-[hsl(var(--foreground))]">
		<header class="glass-strong sticky top-0 z-50">
			<div class="mx-auto flex h-14 max-w-[1200px] items-center justify-between px-4">
				<div class="flex cursor-pointer items-center gap-2" @click="router.push('/')">
					<img src="/logo.png" alt="logo" class="h-8 w-8 rounded" />
					<span class="gradient-text text-lg font-bold">Vistack Admin</span>
				</div>
				<div class="flex items-center gap-3">
					<UiButton variant="ghost" size="sm" @click="router.push('/sensitive-words')">违禁词管理</UiButton>
					<UiButton variant="ghost" size="sm" @click="router.push('/login')">登录</UiButton>
					<UiButton variant="outline" size="sm" @click="router.push('/register')">注册</UiButton>
				</div>
			</div>
		</header>
		<main class="mx-auto max-w-[1200px] px-4 py-6">
			<slot />
		</main>
		<footer class="py-6 text-center text-xs text-muted-foreground">© Vistack</footer>
	</div>
</template>

<style scoped>
</style>
