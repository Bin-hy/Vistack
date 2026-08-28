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
	<div class="flex min-h-[85vh] items-center justify-center px-4">
		<div class="w-full max-w-md">
			<div class="mb-8 flex flex-col items-center text-center">
				<img src="/logo.png" alt="Vistack" class="mb-3 h-14 w-14 rounded-2xl shadow-glow-sm ring-1 ring-white/10" />
				<h1 class="gradient-text text-3xl font-extrabold tracking-tight">创建账号</h1>
				<p class="mt-2 text-sm text-muted-foreground">加入 Vistack，发现更多优质内容</p>
			</div>

			<UiCard class="animate-fade-up p-8">
				<div class="space-y-4">
					<div class="space-y-1.5">
						<label class="block text-sm font-medium text-foreground">用户名</label>
						<UiInput v-model="username" placeholder="请输入用户名" autocomplete="username" />
					</div>
					<div class="space-y-1.5">
						<label class="block text-sm font-medium text-foreground">邮箱 <span class="text-muted-foreground/60">(可选)</span></label>
						<UiInput v-model="email" type="email" placeholder="请输入邮箱" autocomplete="email" />
					</div>
					<div class="space-y-1.5">
						<label class="block text-sm font-medium text-foreground">昵称 <span class="text-muted-foreground/60">(可选)</span></label>
						<UiInput v-model="nickname" placeholder="不填则使用用户名" />
					</div>
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
						<div class="space-y-1.5">
							<label class="block text-sm font-medium text-foreground">密码</label>
							<UiInput v-model="password" type="password" placeholder="请输入密码" autocomplete="new-password" />
						</div>
						<div class="space-y-1.5">
							<label class="block text-sm font-medium text-foreground">确认密码</label>
							<UiInput v-model="confirm" type="password" placeholder="请再次输入" autocomplete="new-password" />
						</div>
					</div>
					<label class="inline-flex cursor-pointer select-none items-center gap-2 text-xs text-muted-foreground">
						<input type="checkbox" v-model="agree" class="h-4 w-4 accent-[hsl(var(--primary))]" />
						<span>我已阅读并同意相关条款与隐私政策</span>
					</label>
					<UiButton class="h-11 w-full" :disabled="loading" @click="onRegister">
						{{ loading ? '注册中…' : '注 册' }}
					</UiButton>
				</div>
				<p class="mt-6 text-center text-sm text-muted-foreground">
					已有账号？
					<router-link to="/login" class="font-medium text-primary hover:underline">去登录</router-link>
				</p>
			</UiCard>
		</div>
	</div>
</template>

<style scoped>
</style>
