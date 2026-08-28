<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton, UiInput } from '@/components/ui'
import { toast } from '@/components/ui/toast/useToast'
import { login as apiLogin } from './api/api'
import { setToken } from '@/lib/http'

const router = useRouter()
const account = ref('')
const password = ref('')
const remember = ref(true)
const loading = ref(false)

async function onLogin() {
  if (!account.value || !password.value) {
    toast({ title: '请输入账号和密码', type: 'error' })
    return
  }
  loading.value = true
  try {
    const res = await apiLogin({ account: account.value, password: password.value })
    setToken(res.token)
    toast({ title: `登录成功，欢迎 ${res.user.nickname ?? res.user.account}`, type: 'success' })
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
    <div class="grid w-full max-w-[960px] grid-cols-1 gap-6 md:grid-cols-2">
      <!-- 左侧品牌视觉块 -->
      <div class="hidden items-center justify-center rounded-xl bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] p-8 shadow-lg md:flex">
        <div class="space-y-2 text-center text-white">
          <img src="/logo.png" alt="logo" class="mx-auto h-16 w-16 rounded-full shadow-md" />
          <h2 class="text-2xl font-bold">欢迎回来</h2>
          <p class="text-sm opacity-90">登录以继续你的精彩内容</p>
        </div>
      </div>

      <!-- 右侧登录表单 -->
      <div class="glass rounded-xl p-8 shadow-lg">
        <h1 class="text-2xl font-bold">登录</h1>
        <p class="mb-6 mt-1 text-sm text-muted-foreground">使用手机号/邮箱登录你的账户</p>

        <div class="space-y-4">
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-right text-sm text-muted-foreground">账号</label>
            <UiInput v-model="account" type="text" placeholder="手机号 / 邮箱" class="flex-1" />
          </div>
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-right text-sm text-muted-foreground">密码</label>
            <UiInput v-model="password" type="password" placeholder="请输入密码" class="flex-1" />
          </div>
          <div class="flex items-center gap-4 text-sm">
            <div class="w-24 shrink-0"></div>
            <div class="flex flex-1 items-center justify-between">
              <label class="inline-flex select-none items-center gap-2">
                <input type="checkbox" v-model="remember" class="h-4 w-4" />
                <span class="text-muted-foreground">记住我</span>
              </label>
              <a href="#" class="text-primary hover:underline">忘记密码？</a>
            </div>
          </div>
          <div class="flex items-center gap-4">
            <div class="w-24 shrink-0"></div>
            <UiButton class="h-11 flex-1" :disabled="loading" @click="onLogin">{{ loading ? '登录中…' : '登录' }}</UiButton>
          </div>
        </div>

        <div class="my-6 flex items-center">
          <div class="h-px flex-1 bg-border"></div>
          <span class="px-3 text-xs text-muted-foreground">或</span>
          <div class="h-px flex-1 bg-border"></div>
        </div>

        <p class="mt-6 text-center text-sm text-muted-foreground">
          还没有账号？
          <router-link to="/register" class="text-primary hover:underline">立即注册</router-link>
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
</style>
