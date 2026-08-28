<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { UiButton, UiInput, UiIcon } from '@/components/ui'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const searchQuery = ref('')
const avatarMenuOpen = ref(false)
const mobileMenuOpen = ref(false)
const scrolled = ref(false)
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

const username = computed(() => userStore.currentUser?.username ?? '')

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

function handleScroll() {
    scrolled.value = window.scrollY > 8
}

onMounted(async () => {
    if (userStore.isLoggedIn && !userStore.currentUser) {
        await userStore.fetchUserInfo()
    }
    document.addEventListener('click', handleClickOutside)
    window.addEventListener('scroll', handleScroll, { passive: true })
    handleScroll()
})

onBeforeUnmount(() => {
    document.removeEventListener('click', handleClickOutside)
    window.removeEventListener('scroll', handleScroll)
})
</script>

<template>
    <div class="flex min-h-screen flex-col bg-[hsl(var(--background))] text-[hsl(var(--foreground))]">
        <header
            class="sticky top-0 z-50 transition-all duration-300"
            :class="
                scrolled
                    ? 'glass-strong shadow-lg shadow-black/30 border-b border-white/10'
                    : 'border-b border-transparent bg-transparent'
            "
        >
            <div class="mx-auto flex h-14 max-w-[1200px] items-center justify-between gap-3 px-4 sm:h-16">
                <!-- Mobile Menu Button -->
                <button
                    class="flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-foreground md:hidden"
                    aria-label="菜单"
                    @click.stop="toggleMobileMenu"
                >
                    <UiIcon name="menu" :size="22" />
                </button>

                <!-- Logo -->
                <div class="flex cursor-pointer select-none items-center gap-2.5" @click="router.push('/')">
                    <img src="/logo.png" alt="Vistack" class="h-8 w-8 rounded-lg ring-1 ring-white/10 sm:h-9 sm:w-9" />
                    <span class="gradient-text hidden text-xl font-extrabold tracking-tight sm:inline-block">Vistack</span>
                </div>

                <!-- Desktop Nav -->
                <nav class="hidden items-center gap-1 text-sm md:flex">
                    <router-link
                        to="/"
                        class="rounded-lg px-3.5 py-2 font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                        :class="route.path === '/' ? 'bg-accent text-foreground' : ''"
                    >
                        首页
                    </router-link>
                    <router-link
                        to="/creator"
                        class="rounded-lg px-3.5 py-2 font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                        :class="route.path === '/creator' ? 'bg-accent text-foreground' : ''"
                    >
                        创作中心
                    </router-link>
                </nav>

                <!-- Search Bar -->
                <div class="hidden max-w-xl flex-1 justify-center px-4 sm:flex">
                    <div class="relative w-full max-w-sm">
                        <UiIcon
                            name="search"
                            :size="16"
                            class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
                        />
                        <UiInput
                            v-model="searchQuery"
                            placeholder="搜索视频、UP主…"
                            class="h-10 pl-9 pr-16 text-sm"
                            @keyup.enter="doSearch"
                        />
                        <UiButton
                            class="absolute right-1 top-1/2 h-8 -translate-y-1/2 px-3 text-xs"
                            size="sm"
                            @click="doSearch"
                        >
                            搜索
                        </UiButton>
                    </div>
                </div>

                <!-- Right Actions -->
                <div class="flex items-center gap-2 sm:gap-3">
                    <!-- Mobile search icon -->
                    <button
                        class="flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-foreground sm:hidden"
                        aria-label="搜索"
                        @click="mobileMenuOpen = true"
                    >
                        <UiIcon name="search" :size="20" />
                    </button>

                    <div ref="avatarWrapperRef" class="relative">
                        <div
                            class="flex h-9 w-9 cursor-pointer select-none items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-sm font-semibold text-white ring-2 ring-white/10 transition-all duration-200 hover:scale-105 hover:ring-primary/50"
                            :title="userStore.displayName || '未登录'"
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

                        <!-- Avatar Dropdown -->
                        <Transition name="dropdown">
                            <div
                                v-if="avatarMenuOpen"
                                class="glass-strong absolute right-0 z-50 mt-2 w-56 rounded-xl p-1.5 shadow-2xl shadow-black/50"
                            >
                                <div class="flex items-center gap-3 rounded-lg px-3 py-2.5">
                                    <div
                                        class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-sm font-semibold text-white"
                                    >
                                        <img
                                            v-if="userStore.currentUser?.avatar_url"
                                            :src="userStore.currentUser?.avatar_url"
                                            alt="avatar"
                                            class="h-full w-full object-cover"
                                        />
                                        <span v-else>{{ avatarText }}</span>
                                    </div>
                                    <div class="min-w-0">
                                        <div class="truncate text-sm font-semibold text-foreground">
                                            {{ userStore.displayName || '未登录' }}
                                        </div>
                                        <div class="truncate text-xs text-muted-foreground">
                                            {{ username || '登录后体验完整功能' }}
                                        </div>
                                    </div>
                                </div>
                                <div class="my-1 h-px bg-white/10"></div>
                                <button
                                    class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-foreground transition-colors hover:bg-accent"
                                    @click="goProfile"
                                >
                                    <UiIcon name="user" :size="16" class="text-muted-foreground" />个人中心
                                </button>
                                <button
                                    class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-foreground transition-colors hover:bg-accent"
                                    @click="onLogout"
                                >
                                    <UiIcon name="logout" :size="16" class="text-muted-foreground" />退出登录
                                </button>
                            </div>
                        </Transition>
                    </div>
                </div>
            </div>

            <!-- Mobile Menu Dropdown -->
            <Transition name="dropdown">
                <div v-if="mobileMenuOpen" class="glass-strong border-t border-white/10 px-4 py-3 md:hidden">
                    <div class="relative mb-3">
                        <UiIcon
                            name="search"
                            :size="16"
                            class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
                        />
                        <UiInput
                            v-model="searchQuery"
                            placeholder="搜索视频、UP主…"
                            class="h-10 pl-9 pr-16 text-sm"
                            @keyup.enter="doSearch"
                        />
                        <UiButton class="absolute right-1 top-1/2 h-8 -translate-y-1/2 px-3 text-xs" size="sm" @click="doSearch">
                            搜索
                        </UiButton>
                    </div>
                    <nav class="flex flex-col gap-1 text-sm">
                        <router-link
                            class="rounded-lg px-3 py-2.5 font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                            :class="route.path === '/' ? 'bg-accent text-foreground' : ''"
                            to="/"
                            @click="mobileMenuOpen = false"
                        >
                            首页
                        </router-link>
                        <router-link
                            class="rounded-lg px-3 py-2.5 font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                            :class="route.path === '/creator' ? 'bg-accent text-foreground' : ''"
                            to="/creator"
                            @click="mobileMenuOpen = false"
                        >
                            创作中心
                        </router-link>
                    </nav>
                </div>
            </Transition>
        </header>

        <main class="mx-auto w-full max-w-[1200px] flex-1 px-4 py-4 sm:py-6">
            <slot />
        </main>

        <footer class="border-t border-white/5 py-8">
            <div class="mx-auto flex max-w-[1200px] flex-col items-center gap-4 px-4 sm:flex-row sm:justify-between">
                <div class="flex items-center gap-2">
                    <img src="/logo.png" alt="Vistack" class="h-6 w-6 rounded" />
                    <span class="gradient-text text-sm font-bold">Vistack</span>
                </div>
                <p class="text-xs text-muted-foreground">© {{ new Date().getFullYear() }} Vistack · 高端视频平台</p>
                <div class="flex items-center gap-4 text-xs text-muted-foreground">
                    <a href="#" class="transition-colors hover:text-foreground">关于我们</a>
                    <a href="#" class="transition-colors hover:text-foreground">用户协议</a>
                    <a href="#" class="transition-colors hover:text-foreground">隐私政策</a>
                </div>
            </div>
        </footer>
    </div>
</template>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
    transition:
        opacity 0.18s ease,
        transform 0.18s ease;
}
.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: translateY(-6px);
}
</style>
