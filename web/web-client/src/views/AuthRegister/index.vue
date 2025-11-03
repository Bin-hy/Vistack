<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton } from '@/components/ui'
import { register as apiRegister } from './api/api'

const router = useRouter()
const account = ref('')
const password = ref('')
const confirm = ref('')
const agree = ref(true)
const loading = ref(false)

async function onRegister() {
  if (!agree.value) {
    alert('请先同意用户协议与隐私政策')
    return
  }
  if (!account.value || !password.value) {
    alert('请输入账号和密码')
    return
  }
  if (password.value !== confirm.value) {
    alert('两次输入的密码不一致')
    return
  }
  loading.value = true
  try {
    const res = await apiRegister({ account: account.value, password: password.value })
    alert(`注册成功，欢迎 ${res.user.nickname ?? res.user.account}`)
    router.push('/login')
  } catch (e: any) {
    alert(e?.message ?? '注册失败，请稍后重试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-[80vh] flex items-center justify-center bg-[hsl(var(--background))]">
    <div class="w-full max-w-[960px] grid grid-cols-1 md:grid-cols-2 gap-6">
      <!-- 左侧视觉块 -->
      <div class="hidden md:flex items-center justify-center rounded-xl bg-gradient-to-br from-blue-400 to-pink-400 p-8 shadow-lg">
        <div class="text-white text-center space-y-2">
          <img src="/logo.png" alt="logo" class="mx-auto h-16 w-16 rounded-full shadow-md" />
          <h2 class="text-2xl font-bold">加入我们</h2>
          <p class="text-sm opacity-90">注册以探索更多精彩内容</p>
        </div>
      </div>

      <!-- 右侧注册表单 -->
      <div class="rounded-xl border border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80 shadow-sm p-8">
        <h1 class="text-2xl font-bold mb-2">注册</h1>
        <p class="text-sm text-gray-500 mb-6">使用手机号/邮箱创建你的账户</p>

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
          <!-- 确认密码 -->
          <div class="flex items-center gap-4">
            <label class="w-24 shrink-0 text-sm text-gray-600 text-right">确认密码</label>
            <input
              v-model="confirm"
              type="password"
              placeholder="请再次输入密码"
              class="flex-1 h-11 px-3 rounded-md border focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <!-- 同意协议 -->
          <div class="flex items-center gap-4 text-sm">
            <div class="w-24 shrink-0"></div>
            <label class="flex-1 inline-flex items-center gap-2 select-none">
              <input type="checkbox" v-model="agree" class="h-4 w-4" />
              我已阅读并同意
              <a href="#" class="text-blue-600 hover:underline">《用户协议》</a>
              与
              <a href="#" class="text-blue-600 hover:underline">《隐私政策》</a>
            </label>
          </div>
          <!-- 注册按钮 -->
          <div class="flex items-center gap-4">
            <div class="w-24 shrink-0"></div>
            <UiButton class="flex-1 h-11" :disabled="loading" @click="onRegister">{{ loading ? '注册中…' : '注册' }}</UiButton>
          </div>
        </div>

        <p class="text-center mt-6 text-sm">
          已有账号？
          <router-link to="/login" class="text-blue-600 hover:underline">去登录</router-link>
        </p>
      </div>
    </div>
  </div>
  
</template>

<style scoped>
</style>