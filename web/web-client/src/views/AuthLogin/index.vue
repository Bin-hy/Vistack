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
	<div class="flex min-h-[80vh] items-center justify-center">
		<UiCard class="w-full max-w-md p-8">
			<h1 class="text-2xl font-bold">登录</h1>
			<p class="mb-6 mt-1 text-sm text-muted-foreground">使用账号和密码登录</p>
			<div class="space-y-4">
				<div class="space-y-1">
					<label class="block text-sm text-muted-foreground">用户名</label>
					<UiInput v-model="username" placeholder="请输入用户名" />
				</div>
				<div class="space-y-1">
					<label class="block text-sm text-muted-foreground">密码</label>
					<UiInput v-model="password" type="password" placeholder="请输入密码" />
				</div>
				<div class="flex items-center justify-between text-xs text-muted-foreground">
					<label class="inline-flex select-none items-center gap-2">
						<input type="checkbox" v-model="remember" class="h-4 w-4" />
						<span>记住我</span>
					</label>
				</div>
				<UiButton class="h-11 w-full" :disabled="loading" @click="onLogin">{{ loading ? '登录中…' : '登录' }}</UiButton>
			</div>
			<p class="mt-6 text-center text-sm text-muted-foreground">
				还没有账号？
				<router-link to="/register" class="text-primary hover:underline">立即注册</router-link>
			</p>
		</UiCard>
	</div>
</template>

<style scoped>
</style>
