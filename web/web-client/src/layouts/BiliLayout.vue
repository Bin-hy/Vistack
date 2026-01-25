<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { UiButton, UiInput } from '@/components/ui'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()

const searchQuery = ref('')
const avatarMenuOpen = ref(false)
const mobileMenuOpen = ref(false)
const avatarWrapperRef = ref<HTMLElement | null>(null)
const mobileMenuRef = ref<HTMLElement | null>(null)

function goProfile() {
    if (!userStore.isLoggedIn) {
        router.push('/login')
        return
    }
    router.push('/profile')
    avatarMenuOpen.value = false
    mobileMenuOpen.value = false
}

function onLogout() {
    userStore.logout()
    router.push('/login')
    avatarMenuOpen.value = false
    mobileMenuOpen.value = false
}

function doSearch() {
    const q = searchQuery.value.trim()
    if (!q) return
    router.push({ name: 'home', query: { q } })
    mobileMenuOpen.value = false
}

const avatarText = computed(() => {
    const name = userStore.displayName || '游客'
    return name.slice(0, 1)
})

function toggleAvatarMenu() {
    avatarMenuOpen.value = !avatarMenuOpen.value
}

function toggleMobileMenu() {
    mobileMenuOpen.value = !mobileMenuOpen.value
}

function handleClickOutside(event: MouseEvent) {
    const target = event.target as Node | null
    
    // Avatar Menu
    if (avatarWrapperRef.value && !avatarWrapperRef.value.contains(target)) {
        avatarMenuOpen.value = false
    }

    // Mobile Menu
    if (mobileMenuRef.value && !mobileMenuRef.value.contains(target)) {
        // Don't close if clicking the toggle button (handled by toggle function)
        // This is a simple check, could be more robust
    }
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
                <!-- Mobile Menu Button -->
                <button class="md:hidden p-2 text-gray-600" @click.stop="toggleMobileMenu">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
                    </svg>
                </button>

                <!-- Logo -->
                <div class="flex items-center gap-2 cursor-pointer" @click="router.push('/')">
                    <img src="/logo.png" alt="logo" class="h-8 w-8 rounded" />
                    <span class="font-bold text-blue-600 hidden sm:inline-block">Vistack</span>
                </div>

                <!-- Desktop Nav -->
                <nav class="hidden md:flex items-center gap-3 text-sm text-gray-700">
                    <router-link class="hover:text-blue-600" to="/">首页</router-link>
                    <router-link class="hover:text-blue-600" to="/">动画</router-link>
                    <router-link class="hover:text-blue-600" to="/">游戏</router-link>
                    <router-link class="hover:text-blue-600" to="/">影视</router-link>
                    <router-link class="hover:text-blue-600" to="/">直播</router-link>
                </nav>

                <!-- Search Bar (Desktop & Tablet) -->
                <div class="hidden sm:flex flex-1 max-w-xl justify-center px-4">
                    <div class="flex w-full items-center gap-2">
                        <UiInput
                            v-model="searchQuery"
                            placeholder="搜索"
                            class="h-9 w-full text-sm"
                            @keyup.enter="doSearch"
                        />
                        <UiButton class="h-9 px-3" size="sm" @click="doSearch">搜索</UiButton>
                    </div>
                </div>

                <!-- Right Actions -->
                <div class="flex items-center gap-2 sm:gap-3">
                    <!-- Mobile Search Icon (TODO: Expandable search) -->
                    
                    <div class="hidden md:flex items-center gap-2">
                         <button class="h-8 w-8 rounded-full bg-gray-100 flex items-center justify-center text-xs hover:bg-gray-200" title="消息">🔔</button>
                         <button class="h-8 w-8 rounded-full bg-gray-100 flex items-center justify-center text-xs hover:bg-gray-200" title="动态">📜</button>
                         <button class="h-8 w-8 rounded-full bg-gray-100 flex items-center justify-center text-xs hover:bg-gray-200" title="收藏">⭐</button>
                         <button class="h-8 w-8 rounded-full bg-gray-100 flex items-center justify-center text-xs hover:bg-gray-200" title="历史">🕘</button>
                    </div>

                    <UiButton variant="outline" size="sm" class="hidden md:inline-flex h-8 text-xs" @click="router.push('/creator')">创作中心</UiButton>
                    
                    <div ref="avatarWrapperRef" class="relative">
                        <div
                            class="h-8 w-8 sm:h-9 sm:w-9 rounded-full overflow-hidden ring-1 ring-gray-200 flex items-center justify-center bg-gradient-to-br from-blue-100 to-purple-100 text-gray-700 cursor-pointer transition-transform duration-150 hover:scale-110"
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
                            class="absolute right-0 mt-2 w-44 rounded-md border border-[hsl(var(--border))] bg-white shadow-lg z-50"
                        >
                            <div class="py-1 text-sm">
                                <button class="flex w-full items-center gap-2 px-3 py-2 hover:bg-gray-50" @click="goProfile">个人中心</button>
                                <button class="flex w-full items-center gap-2 px-3 py-2 hover:bg-gray-50 md:hidden" @click="router.push('/creator')">创作中心</button>
                                <button class="flex w-full items-center gap-2 px-3 py-2 hover:bg-gray-50" @click="onLogout">退出登录</button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            
            <!-- Mobile Menu Dropdown -->
            <div v-if="mobileMenuOpen" ref="mobileMenuRef" class="md:hidden border-t border-gray-100 bg-white px-4 py-2 shadow-lg">
                <div class="flex items-center gap-2 mb-4">
                    <UiInput
                        v-model="searchQuery"
                        placeholder="搜索"
                        class="h-9 w-full text-sm"
                        @keyup.enter="doSearch"
                    />
                    <UiButton class="h-9 px-3" size="sm" @click="doSearch">Go</UiButton>
                </div>
                <nav class="flex flex-col gap-2 text-sm text-gray-700">
                    <router-link class="py-2 px-2 hover:bg-gray-50 rounded" to="/" @click="mobileMenuOpen = false">首页</router-link>
                    <router-link class="py-2 px-2 hover:bg-gray-50 rounded" to="/" @click="mobileMenuOpen = false">动画</router-link>
                    <router-link class="py-2 px-2 hover:bg-gray-50 rounded" to="/" @click="mobileMenuOpen = false">游戏</router-link>
                    <router-link class="py-2 px-2 hover:bg-gray-50 rounded" to="/" @click="mobileMenuOpen = false">影视</router-link>
                    <router-link class="py-2 px-2 hover:bg-gray-50 rounded" to="/" @click="mobileMenuOpen = false">直播</router-link>
                </nav>
            </div>
        </header>
        <main class="max-w-[1200px] mx-auto px-4 py-4 sm:py-6">
            <slot />
        </main>
        <footer class="py-6 text-center text-xs text-gray-500">© Vistack</footer>
    </div>
</template>

<style scoped>
</style>
