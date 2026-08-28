<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton, UiInput } from '@/components/ui'
import { toast } from '@/components/ui/toast/useToast'
import { register as apiRegister } from './api/api'

const router = useRouter()
const username = ref('')
const password = ref('')
const confirm = ref('')
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
    const res = await apiRegister({ username: username.value, password: password.value })
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
    <div class="grid w-full max-w-[960px] grid-cols-1 gap-6 md:grid-cols-2">
      <!-- 左侧品牌视觉块 -->
      <div class="hidden items-center justify-center rounded-xl bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] p-8 shadow-lg md:flex">
        <div class="space-y-2 text-center text-white">
          <img src="/logo.png" alt="logo" class="mx-auto h-16 w-16 rounded-full shadow-md" />
          <h2 class="text-2xl font-bold">加入我们</h2>
          <p class="text-sm opacity-90">注册以探索更多精彩内容</p>
        </div>
      </div>

      <!-- 右侧注册表单 -->
      <div class="glass rounded-xl p-8 shadow-lg">
        <h1 class="text-2xl font-bold">注册</h1>
        <p class="mb-6 mt-1 text-sm text-muted-foreground">使用用户名和密码创建账户</p>

        <div class="space-y-4">
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-right text-sm text-muted-foreground">用户名</label>
            <UiInput v-model="username" type="text" placeholder="请输入用户名" class="flex-1" />
          </div>
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-right text-sm text-muted-foreground">密码</label>
            <UiInput v-model="password" type="password" placeholder="请输入密码" class="flex-1" />
          </div>
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-right text-sm text-muted-foreground">确认密码</label>
            <UiInput v-model="confirm" type="password" placeholder="请再次输入密码" class="flex-1" />
          </div>
          <div class="flex items-center gap-4 text-sm">
            <div class="w-24 shrink-0"></div>
            <label class="flex flex-1 select-none items-center gap-2 text-muted-foreground">
              <input type="checkbox" v-model="agree" class="h-4 w-4" />
              <span>我已阅读并同意</span>
              <a href="#" class="text-primary hover:underline">《用户协议》</a>
              <span>与</span>
              <a href="#" class="text-primary hover:underline">《隐私政策》</a>
            </label>
          </div>
          <div class="flex items-center gap-4">
            <div class="w-24 shrink-0"></div>
            <UiButton class="h-11 flex-1" :disabled="loading" @click="onRegister">{{ loading ? '注册中…' : '注册' }}</UiButton>
          </div>
        </div>

        <p class="mt-6 text-center text-sm text-muted-foreground">
          已有账号？
          <router-link to="/login" class="text-primary hover:underline">去登录</router-link>
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
</style>
