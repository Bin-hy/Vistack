<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { UiButton, UiInput } from '@/components/ui'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()

const searchQuery = ref('')
const avatarMenuOpen = ref(false)
const avatarWrapperRef = ref<HTMLElement | null>(null)

function goProfile() {
    if (!userStore.isLoggedIn) {
        router.push('/login')
        return
    }
    router.push('/profile')
}

function onLogout() {
    userStore.logout()
    router.push('/login')
}

function doSearch() {
    const q = searchQuery.value.trim()
    if (!q) return
    router.push({ name: 'home', query: { q } })
}

const avatarText = computed(() => {
    const name = userStore.displayName || '游客'
    return name.slice(0, 1)
})

function toggleAvatarMenu() {
    avatarMenuOpen.value = !avatarMenuOpen.value
}

function handleClickOutside(event: MouseEvent) {
    const el = avatarWrapperRef.value
    if (!el) return
    const target = event.target as Node | null
    if (target && el.contains(target)) return
    avatarMenuOpen.value = false
}

onMounted(async () => {
    if (userStore.isLoggedIn && !userStore.currentUser) {
        await userStore.fetchUserInfo()
    }
    document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
    document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
    <div class="min-h-screen bg-[hsl(var(--background))] text-[hsl(var(--foreground))]">
        <header class="sticky top-0 z-50 border-b border-[hsl(var(--border))] bg-white/90 backdrop-blur dark:bg-neutral-900/80">
            <div class="max-w-[1200px] mx-auto px-4 h-14 flex items-center justify-between gap-4">
                <div class="flex items-center gap-4">
                    <div class="flex items-center gap-2">
                        <img src="/logo.png" alt="logo" class="h-8 w-8 rounded" />
                        <span class="font-bold text-blue-600">Vistack</span>
                    </div>
                    <nav class="hidden md:flex items-center gap-3 text-sm text-gray-700">
                        <router-link class="hover:text-blue-600" to="/">首页</router-link>
                        <router-link class="hover:text-blue-600" to="/">动画</router-link>
                        <router-link class="hover:text-blue-600" to="/">游戏</router-link>
                        <router-link class="hover:text-blue-600" to="/">影视</router-link>
                        <router-link class="hover:text-blue-600" to="/">直播</router-link>
                    </nav>
                </div>
                <div class="flex-1 max-w-xl">
                    <div class="flex items-center gap-2">
                        <UiInput
                            v-model="searchQuery"
                            placeholder="搜索视频、UP主、番剧"
                            class="h-10"
                            @keyup.enter="doSearch"
                        />
                        <UiButton class="h-10 px-4" @click="doSearch">搜索</UiButton>
                    </div>
                </div>
                <div class="flex items-center gap-3">
                    <button class="h-9 w-9 rounded-full bg-gray-100 flex items-center justify-center text-sm hover:bg-gray-200" title="消息">🔔</button>
                    <button class="h-9 w-9 rounded-full bg-gray-100 flex items-center justify-center text-sm hover:bg-gray-200" title="动态">📜</button>
                    <button class="h-9 w-9 rounded-full bg-gray-100 flex items-center justify-center text-sm hover:bg-gray-200" title="收藏">⭐</button>
                    <button class="h-9 w-9 rounded-full bg-gray-100 flex items-center justify-center text-sm hover:bg-gray-200" title="历史">🕘</button>
                    <UiButton variant="outline" size="sm" class="hidden md:inline-flex" @click="router.push('/creator')">创作中心</UiButton>
                    <div ref="avatarWrapperRef" class="relative">
                        <div
                            class="h-9 w-9 rounded-full overflow-hidden ring-1 ring-gray-200 flex items-center justify-center bg-gradient-to-br from-blue-100 to-purple-100 text-gray-700 cursor-pointer transition-transform duration-150 hover:scale-110"
                            @click="toggleAvatarMenu"
                        >
                            <template v-if="userStore.currentUser?.avatar_url">
                                <img :src="userStore.currentUser?.avatar_url" alt="avatar" class="h-full w-full object-cover" />
                            </template>
                            <template v-else>
                                <span class="text-sm font-medium">{{ avatarText }}</span>
                            </template>
                        </div>
                        <div
                            v-if="avatarMenuOpen"
                            class="absolute right-0 mt-2 w-44 rounded-md border border-[hsl(var(--border))] bg-white shadow-lg"
                        >
                            <div class="py-1 text-sm">
                                <button class="flex w-full items-center gap-2 px-3 py-2 hover:bg-gray-50" @click="goProfile">个人中心</button>
                                <button class="flex w-full items-center gap-2 px-3 py-2 hover:bg-gray-50" @click="router.push('/creator')">创作中心</button>
                                <button class="flex w-full items-center gap-2 px-3 py-2 hover:bg-gray-50" @click="onLogout">退出登录</button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </header>
        <main class="max-w-[1200px] mx-auto px-4 py-6">
            <slot />
        </main>
        <footer class="py-6 text-center text-xs text-gray-500">© Vistack</footer>
    </div>
</template>

<style scoped>
</style>
