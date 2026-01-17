<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton, UiCard, UiInput } from '@/components/ui'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()
const username = ref('')
const password = ref('')
const remember = ref(true)
const loading = ref(false)

async function onLogin() {
	if (!username.value || !password.value) {
		alert('请输入账号和密码')
		return
	}
	loading.value = true
	try {
		await userStore.login(username.value, password.value)
		if (!remember.value) {
		}
		const displayName = userStore.displayName
		alert(`登录成功，欢迎 ${displayName}`)
    router.push('/')
  } catch (e: any) {
    alert(e?.message ?? '登录失败，请稍后重试')
  } finally {
    loading.value = false
	}
}
</script>

<template>
	<div class="min-h-[80vh] flex items-center justify-center bg-[hsl(var(--background))]">
		<UiCard class="w-full max-w-md p-8">
			<h1 class="text-2xl font-bold mb-2">登录</h1>
			<p class="text-sm text-gray-500 mb-6">使用账号和密码登录</p>
			<div class="space-y-4">
				<div class="space-y-1">
					<label class="block text-sm text-gray-600">用户名</label>
					<UiInput v-model="username" placeholder="请输入用户名" />
				</div>
				<div class="space-y-1">
					<label class="block text-sm text-gray-600">密码</label>
					<UiInput v-model="password" type="password" placeholder="请输入密码" />
				</div>
				<div class="flex items-center justify-between text-xs text-gray-500">
					<label class="inline-flex items-center gap-2 select-none">
						<input type="checkbox" v-model="remember" class="h-4 w-4" />
						<span>记住我</span>
					</label>
				</div>
				<UiButton class="w-full h-11" :disabled="loading" @click="onLogin">{{ loading ? '登录中…' : '登录' }}</UiButton>
			</div>
			<p class="text-center mt-6 text-sm">
				还没有账号？
				<router-link to="/register" class="text-blue-600 hover:underline">立即注册</router-link>
			</p>
		</UiCard>
	</div>
</template>

<style scoped>
</style>
