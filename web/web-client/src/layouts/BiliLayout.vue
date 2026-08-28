<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { UiButton, UiInput, UiIcon } from '@/components/ui'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()

const searchQuery = ref('')
const avatarMenuOpen = ref(false)
const mobileMenuOpen = ref(false)
const avatarWrapperRef = ref<HTMLElement | null>(null)

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
    if (avatarWrapperRef.value && !avatarWrapperRef.value.contains(target)) {
        avatarMenuOpen.value = false
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
        <header class="glass-strong sticky top-0 z-50">
            <div class="mx-auto flex h-14 max-w-[1200px] items-center justify-between gap-3 px-4">
                <!-- Mobile Menu Button -->
                <button class="p-2 text-muted-foreground md:hidden" @click.stop="toggleMobileMenu">
                    <UiIcon name="menu" :size="22" />
                </button>

                <!-- Logo -->
                <div class="flex cursor-pointer items-center gap-2" @click="router.push('/')">
                    <img src="/logo.png" alt="logo" class="h-8 w-8 rounded" />
                    <span class="gradient-text hidden text-lg font-bold sm:inline-block">Vistack</span>
                </div>

                <!-- Desktop Nav -->
                <nav class="hidden items-center gap-1 text-sm md:flex">
                    <router-link
                        class="rounded-md px-3 py-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                        to="/"
                    >
                        首页
                    </router-link>
                </nav>

                <!-- Search Bar -->
                <div class="hidden max-w-xl flex-1 justify-center px-4 sm:flex">
                    <div class="flex w-full items-center gap-2">
                        <UiInput v-model="searchQuery" placeholder="搜索" class="h-9 text-sm" @keyup.enter="doSearch" />
                        <UiButton class="h-9 px-3" size="sm" @click="doSearch">搜索</UiButton>
                    </div>
                </div>

                <!-- Right Actions -->
                <div class="flex items-center gap-2 sm:gap-3">
                    <UiButton
                        variant="outline"
                        size="sm"
                        class="hidden h-8 text-xs md:inline-flex"
                        @click="router.push('/creator')"
                    >
                        创作中心
                    </UiButton>

                    <div ref="avatarWrapperRef" class="relative">
                        <div
                            class="flex h-8 w-8 cursor-pointer items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-sm font-medium text-white ring-1 ring-white/20 transition-transform duration-150 hover:scale-110 sm:h-9 sm:w-9"
                            @click="toggleAvatarMenu"
                        >
                            <img
                                v-if="userStore.currentUser?.avatar_url"
                                :src="userStore.currentUser?.avatar_url"
                                alt="avatar"
                                class="h-full w-full object-cover"
                            />
                            <span v-else>{{ avatarText }}</span>
                        </div>
                        <div
                            v-if="avatarMenuOpen"
                            class="glass-strong absolute right-0 z-50 mt-2 w-44 rounded-lg p-1 shadow-xl shadow-black/30"
                        >
                            <button
                                class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground hover:bg-accent"
                                @click="goProfile"
                            >
                                <UiIcon name="user" :size="16" />个人中心
                            </button>
                            <button
                                class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground hover:bg-accent"
                                @click="onLogout"
                            >
                                <UiIcon name="logout" :size="16" />退出登录
                            </button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Mobile Menu Dropdown -->
            <div v-if="mobileMenuOpen" class="glass-strong border-t-0 px-4 py-3 md:hidden">
                <div class="mb-3 flex items-center gap-2">
                    <UiInput v-model="searchQuery" placeholder="搜索" class="h-9 text-sm" @keyup.enter="doSearch" />
                    <UiButton class="h-9 px-3" size="sm" @click="doSearch">搜索</UiButton>
                </div>
                <nav class="flex flex-col gap-1 text-sm">
                    <router-link class="rounded-md px-2 py-2 text-muted-foreground hover:bg-accent hover:text-foreground" to="/" @click="mobileMenuOpen = false">首页</router-link>
                    <router-link class="rounded-md px-2 py-2 text-muted-foreground hover:bg-accent hover:text-foreground" to="/creator" @click="mobileMenuOpen = false">创作中心</router-link>
                </nav>
            </div>
        </header>

        <main class="mx-auto max-w-[1200px] px-4 py-4 sm:py-6">
            <slot />
        </main>
        <footer class="py-6 text-center text-xs text-muted-foreground">© Vistack</footer>
    </div>
</template>

<style scoped>
</style>
