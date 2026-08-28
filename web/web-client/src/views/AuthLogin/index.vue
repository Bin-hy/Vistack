<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton, UiCard, UiInput } from '@/components/ui'
import { toast } from '@/components/ui/toast/useToast'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()
const username = ref('')
const password = ref('')
const remember = ref(true)
const loading = ref(false)

async function onLogin() {
	if (!username.value || !password.value) {
		toast({ title: '请输入账号和密码', type: 'error' })
		return
	}
	loading.value = true
	try {
		await userStore.login(username.value, password.value)
		toast({ title: `登录成功，欢迎 ${userStore.displayName}`, type: 'success' })
		router.push('/')
	} catch (e: any) {
		toast({ title: e?.message ?? '登录失败，请稍后重试', type: 'error' })
	} finally {
		loading.value = false
	}
}
</script>

<template>
	<div class="flex min-h-[85vh] items-center justify-center px-4">
		<div class="w-full max-w-md">
			<div class="mb-8 flex flex-col items-center text-center">
				<img src="/logo.png" alt="Vistack" class="mb-3 h-14 w-14 rounded-2xl shadow-glow-sm ring-1 ring-white/10" />
				<h1 class="gradient-text text-3xl font-extrabold tracking-tight">Vistack</h1>
				<p class="mt-2 text-sm text-muted-foreground">登录后开启你的高端观影体验</p>
			</div>

			<UiCard class="animate-fade-up p-8">
				<div class="space-y-4">
					<div class="space-y-1.5">
						<label class="block text-sm font-medium text-foreground">用户名</label>
						<UiInput v-model="username" placeholder="请输入用户名" autocomplete="username" />
					</div>
					<div class="space-y-1.5">
						<label class="block text-sm font-medium text-foreground">密码</label>
						<UiInput v-model="password" type="password" placeholder="请输入密码" autocomplete="current-password" @keyup.enter="onLogin" />
					</div>
					<div class="flex items-center justify-between text-xs text-muted-foreground">
						<label class="inline-flex cursor-pointer select-none items-center gap-2">
							<input type="checkbox" v-model="remember" class="h-4 w-4 accent-[hsl(var(--primary))]" />
							<span>记住我</span>
						</label>
						<a href="#" class="text-primary hover:underline">忘记密码？</a>
					</div>
					<UiButton class="h-11 w-full" :disabled="loading" @click="onLogin">
						{{ loading ? '登录中…' : '登 录' }}
					</UiButton>
				</div>
				<div class="my-6 flex items-center gap-3 text-xs text-muted-foreground/60">
					<span class="h-px flex-1 bg-border"></span>
					或
					<span class="h-px flex-1 bg-border"></span>
				</div>
				<p class="text-center text-sm text-muted-foreground">
					还没有账号？
					<router-link to="/register" class="font-medium text-primary hover:underline">立即注册</router-link>
				</p>
			</UiCard>
		</div>
	</div>
</template>

<style scoped>
</style>
