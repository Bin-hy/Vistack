<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton, UiCard, UiInput } from '@/components/ui'
import { toast } from '@/components/ui/toast/useToast'
import { register as apiRegister } from './api/api'

const router = useRouter()
const username = ref('')
const password = ref('')
const confirm = ref('')
const email = ref('')
const nickname = ref('')
const agree = ref(true)
const loading = ref(false)

async function onRegister() {
  if (!agree.value) {
    toast({ title: '请先同意用户协议与隐私政策', type: 'error' })
    return
  }
	if (!username.value || !password.value) {
		toast({ title: '请输入用户名和密码', type: 'error' })
    return
  }
  if (password.value !== confirm.value) {
    toast({ title: '两次输入的密码不一致', type: 'error' })
    return
	}
	loading.value = true
	try {
		const res = await apiRegister({
			username: username.value,
			password: password.value,
			email: email.value || undefined,
			nickname: nickname.value || undefined,
		})
		toast({ title: `注册成功，用户ID：${res.user_id}`, type: 'success' })
    router.push('/login')
  } catch (e: any) {
    toast({ title: e?.message ?? '注册失败，请稍后重试', type: 'error' })
  } finally {
    loading.value = false
  }
}
</script>

<template>
	<div class="flex min-h-[80vh] items-center justify-center">
		<UiCard class="w-full max-w-md p-8">
			<h1 class="text-2xl font-bold">注册</h1>
			<p class="mb-6 mt-1 text-sm text-muted-foreground">创建新账号</p>
			<div class="space-y-4">
				<div class="space-y-1">
					<label class="block text-sm text-muted-foreground">用户名</label>
					<UiInput v-model="username" placeholder="请输入用户名" />
				</div>
				<div class="space-y-1">
					<label class="block text-sm text-muted-foreground">邮箱（可选）</label>
					<UiInput v-model="email" type="email" placeholder="请输入邮箱" />
				</div>
				<div class="space-y-1">
					<label class="block text-sm text-muted-foreground">昵称（可选）</label>
					<UiInput v-model="nickname" placeholder="请输入昵称，不填则使用用户名" />
				</div>
				<div class="space-y-1">
					<label class="block text-sm text-muted-foreground">密码</label>
					<UiInput v-model="password" type="password" placeholder="请输入密码" />
				</div>
				<div class="space-y-1">
					<label class="block text-sm text-muted-foreground">确认密码</label>
					<UiInput v-model="confirm" type="password" placeholder="请再次输入密码" />
				</div>
				<div class="flex items-center text-xs text-muted-foreground">
					<label class="inline-flex select-none items-center gap-2">
						<input type="checkbox" v-model="agree" class="h-4 w-4" />
						<span>我已阅读并同意相关条款</span>
					</label>
				</div>
				<UiButton class="h-11 w-full" :disabled="loading" @click="onRegister">{{ loading ? '注册中…' : '注册' }}</UiButton>
			</div>
			<p class="mt-6 text-center text-sm text-muted-foreground">
				已有账号？
				<router-link to="/login" class="text-primary hover:underline">去登录</router-link>
			</p>
		</UiCard>
	</div>
</template>

<style scoped>
</style>
