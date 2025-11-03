<script setup lang="ts">
// B站风格首页布局：顶部导航 + 三栏布局（左菜单 / 中心内容 / 右侧榜单）
import { UiButton } from '@/components/ui'
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div class="min-h-screen bg-[hsl(var(--background))] text-[hsl(var(--foreground))]">
    <!-- 顶部导航栏 -->
    <header class="sticky top-0 z-30 bg-white/90 dark:bg-neutral-900/80 backdrop-blur border-b border-[hsl(var(--border))]">
      <div class="max-w-[1200px] mx-auto px-4 h-14 flex items-center gap-4">
        <!-- 左：Logo -->
        <div class="flex items-center gap-2">
          <img src="/logo.png" alt="logo" class="h-8 w-8 rounded" />
          <span class="font-bold text-blue-600">Vistack</span>
        </div>
        <!-- 中：主导航 -->
        <nav class="hidden md:flex items-center gap-3 text-sm">
          <a href="#" class="hover:text-blue-600">首页</a>
          <a href="#" class="hover:text-blue-600">动画</a>
          <a href="#" class="hover:text-blue-600">番剧</a>
          <a href="#" class="hover:text-blue-600">直播</a>
          <a href="#" class="hover:text-blue-600">游戏</a>
          <a href="#" class="hover:text-blue-600">音乐</a>
          <a href="#" class="hover:text-blue-600">科技</a>
        </nav>
        <!-- 搜索框 -->
        <div class="ml-auto flex-1 max-w-md">
          <div class="relative">
            <input
              type="text"
              placeholder="搜索视频、番剧、UP主"
              class="w-full h-10 rounded-full border border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80 pl-10 pr-12 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <div class="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400">🔍</div>
            <button class="absolute right-1 top-1/2 -translate-y-1/2 px-3 h-8 rounded-full bg-blue-500 hover:bg-blue-600 text-white text-sm">搜索</button>
          </div>
        </div>
        <!-- 右：操作区 -->
        <div class="hidden md:flex items-center gap-3">
          <UiButton variant="link" size="sm" class="!text-blue-600 !no-underline" @click="router.push('/login')">登录</UiButton>
          <UiButton variant="link" size="sm" class="!text-blue-600 !no-underline" @click="router.push('/register')">注册</UiButton>
          <UiButton size="sm" class="rounded-full !bg-pink-500 !hover:bg-pink-600 !text-white">投稿</UiButton>
        </div>
      </div>
    </header>

    <!-- 主体三栏布局 -->
    <main class="max-w-[1200px] mx-auto px-4 py-4 grid grid-cols-1 md:grid-cols-[200px_minmax(0,1fr)_260px] gap-4">
      <!-- 左侧菜单 -->
      <aside class="hidden md:block">
        <div class="rounded-lg border border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80 p-3">
          <h3 class="text-sm font-semibold mb-2">频道</h3>
          <ul class="space-y-1 text-sm">
            <li><a href="#" class="block px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-neutral-800">动画</a></li>
            <li><a href="#" class="block px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-neutral-800">番剧</a></li>
            <li><a href="#" class="block px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-neutral-800">国创</a></li>
            <li><a href="#" class="block px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-neutral-800">音乐</a></li>
            <li><a href="#" class="block px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-neutral-800">舞蹈</a></li>
            <li><a href="#" class="block px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-neutral-800">科技</a></li>
            <li><a href="#" class="block px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-neutral-800">生活</a></li>
          </ul>
        </div>
      </aside>

      <!-- 中心内容区 -->
      <section>
        <!-- 顶部横幅/轮播占位 -->
        <div class="rounded-lg overflow-hidden mb-4 bg-gradient-to-r from-blue-500 to-cyan-500 h-28"></div>
        <!-- 内容网格：外部通过默认插槽传入卡片 -->
        <div class="grid grid-cols-2 lg:grid-cols-3 gap-4">
          <slot />
        </div>
      </section>

      <!-- 右侧榜单/登录引导 -->
      <aside class="hidden md:block">
        <div class="rounded-lg border border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80 p-3">
          <h3 class="text-sm font-semibold mb-2">排行榜</h3>
          <ul class="space-y-2 text-sm">
            <li class="flex items-center gap-2">
              <span class="w-5 text-center text-blue-600">1</span>
              <a href="#" class="flex-1 hover:text-blue-600 truncate">爆款视频标题示例</a>
            </li>
            <li class="flex items-center gap-2">
              <span class="w-5 text-center text-blue-600">2</span>
              <a href="#" class="flex-1 hover:text-blue-600 truncate">热门番剧更新今日看点</a>
            </li>
            <li class="flex items-center gap-2">
              <span class="w-5 text-center text-blue-600">3</span>
              <a href="#" class="flex-1 hover:text-blue-600 truncate">科技区新鲜事</a>
            </li>
          </ul>
        </div>

        <div class="mt-4 rounded-lg border border-[hsl(var(--border))] bg-white/90 dark:bg-neutral-900/80 p-3">
          <h3 class="text-sm font-semibold mb-2">登录提示</h3>
          <p class="text-xs text-gray-500 mb-2">登录后可同步订阅、历史记录</p>
          <div class="flex gap-2">
            <UiButton class="flex-1 h-9" @click="router.push('/login')">登录</UiButton>
            <UiButton variant="outline" class="flex-1 h-9" @click="router.push('/register')">注册</UiButton>
          </div>
        </div>
      </aside>
    </main>

    <!-- 底部 -->
    <footer class="py-8 text-center text-xs text-gray-500">© Vistack</footer>
  </div>
</template>

<style scoped>
</style>
