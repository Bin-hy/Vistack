<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton } from '@/components/ui'
import { login as apiLogin } from './api/api'
import { setToken } from '@/lib/http'

const router = useRouter()
const account = ref('')
const password = ref('')
const remember = ref(true)
const loading = ref(false)

async function onLogin() {
  if (!account.value || !password.value) {
    alert('请输入账号和密码')
    return
  }
  loading.value = true
  try {
    const res = await apiLogin({ account: account.value, password: password.value })
    setToken(res.token)
    // 可选：记住我逻辑
    if (!remember.value) {
      // 简化处理：不记住则只保存在内存（此处保持 localStorage，实际项目可换 sessionStorage）
    }
    alert(`登录成功，欢迎 ${res.user.nickname ?? res.user.account}`)
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
    <div class="w-full max-w-[960px] grid grid-cols-1 md:grid-cols-2 gap-6">
      <!-- 左侧视觉块（仿 B站风格：简洁插图/品牌区） -->
      <div class="hidden md:flex items-center justify-center rounded-xl bg-gradient-to-br from-blue-400 to-pink-400 p-8 shadow-lg">
        <div class="text-white text-center space-y-2">
          <img src="/logo.png" alt="logo" class="mx-auto h-16 w-16 rounded-full shadow-md" />
          <h2 class="text-2xl font-bold">欢迎回来</h2>
          <p class="text-sm opacity-90">登录以继续你的精彩内容</p>
        </div>
      </div>

      <!-- 右侧登录表单 -->
      <div class="rounded-xl border border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80 shadow-sm p-8">
        <h1 class="text-2xl font-bold mb-2">登录</h1>
        <p class="text-sm text-gray-500 mb-6">使用手机号/邮箱登录你的账户</p>

        <div class="space-y-4">
          <!-- 账号 -->
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-sm text-gray-600 text-right">账号</label>
            <input
              v-model="account"
              type="text"
              placeholder="手机号 / 邮箱"
              class="flex-1 h-11 px-3 rounded-md border focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <!-- 密码 -->
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-sm text-gray-600 text-right">密码</label>
            <input
              v-model="password"
              type="password"
              placeholder="请输入密码"
              class="flex-1 h-11 px-3 rounded-md border focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <!-- 记住我 / 忘记密码 -->
          <div class="flex items-center gap-4 text-sm">
            <div class="w-24 shrink-0"></div>
            <div class="flex-1 flex items-center justify-between">
              <label class="inline-flex items-center gap-2 select-none">
                <input type="checkbox" v-model="remember" class="h-4 w-4" />
                记住我
              </label>
              <a href="#" class="text-blue-600 hover:underline">忘记密码？</a>
            </div>
          </div>
          <!-- 登录按钮 -->
          <div class="flex items-center gap-4">
            <div class="w-24 shrink-0"></div>
            <UiButton class="flex-1 h-11" :disabled="loading" @click="onLogin">{{ loading ? '登录中…' : '登录' }}</UiButton>
          </div>
        </div>

        <div class="my-6 flex items-center">
          <div class="flex-1 h-px bg-gray-200"></div>
          <span class="px-3 text-xs text-gray-500">或</span>
          <div class="flex-1 h-px bg-gray-200"></div>
        </div>

        <!-- 第三方登录（占位） -->
        <div class="flex items-center justify-center gap-4">
          <button class="h-10 w-10 rounded-full bg-gray-100 hover:bg-gray-200"></button>
          <button class="h-10 w-10 rounded-full bg-gray-100 hover:bg-gray-200"></button>
          <button class="h-10 w-10 rounded-full bg-gray-100 hover:bg-gray-200"></button>
        </div>

        <p class="text-center mt-6 text-sm">
          还没有账号？
          <router-link to="/register" class="text-blue-600 hover:underline">立即注册</router-link>
        </p>
      </div>
    </div>
  </div>
  
</template>

<style scoped>
</style>