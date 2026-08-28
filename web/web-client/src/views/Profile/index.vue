<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useUserStore } from '@/stores/user'
import BiliLayout from '@/layouts/BiliLayout.vue'
import { UiButton, UiCard, UiInput, UiAvatarUpload } from '@/components/ui'
import { toast } from '@/components/ui/toast/useToast'

const userStore = useUserStore()
const nickname = ref('')
const avatarFile = ref<File | null>(null)
const saving = ref(false)

const username = computed(() => userStore.currentUser?.username ?? '')
const email = computed(() => userStore.currentUser?.email ?? '')

const avatarPreviewUrl = computed(() => {
	return userStore.currentUser?.avatar_url ?? null
})

onMounted(async () => {
	if (userStore.isLoggedIn && !userStore.currentUser) {
		await userStore.fetchUserInfo()
	}
	nickname.value = userStore.currentUser?.nickname ?? ''
})

function onAvatarChange(file: File | null) {
	avatarFile.value = file
}

async function onSave() {
	if (!userStore.isLoggedIn) {
		toast({ title: '请先登录', type: 'error' })
		return
	}
	saving.value = true
	try {
		await userStore.updateProfileWithAvatar({
			nickname: nickname.value || undefined,
			avatarFile: avatarFile.value,
		})
		toast({ title: '资料已更新', type: 'success' })
		avatarFile.value = null
	} catch (e: any) {
		toast({ title: e?.message ?? '更新失败，请稍后重试', type: 'error' })
	} finally {
		saving.value = false
	}
}
</script>

<template>
	<BiliLayout>
		<div class="animate-fade-in mx-auto max-w-2xl">
			<h1 class="mb-5 text-xl font-semibold">个人资料</h1>
			<UiCard class="p-6 sm:p-8">
				<div class="mb-6 flex items-center gap-4 border-b border-border pb-6">
					<img
						v-if="avatarPreviewUrl"
						:src="avatarPreviewUrl"
						alt="avatar"
						class="h-16 w-16 rounded-full object-cover ring-2 ring-primary/30"
					/>
					<div
						v-else
						class="flex h-16 w-16 items-center justify-center rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-2xl font-bold text-white ring-2 ring-primary/30"
					>
						{{ (userStore.displayName || 'V')[0] }}
					</div>
					<div>
						<div class="text-lg font-semibold text-foreground">{{ userStore.displayName || '未设置昵称' }}</div>
						<div class="text-sm text-muted-foreground">@{{ username || '-' }}</div>
					</div>
				</div>

				<div class="space-y-5">
					<div>
						<div class="mb-1.5 text-sm font-medium text-foreground">用户名</div>
						<div class="text-sm text-muted-foreground">{{ username || '未设置' }}</div>
					</div>
					<div v-if="email" class="space-y-1.5">
						<label class="block text-sm font-medium text-foreground">邮箱</label>
						<div class="text-sm text-muted-foreground">{{ email }}</div>
					</div>
					<div class="space-y-1.5">
						<label class="block text-sm font-medium text-foreground">昵称</label>
						<UiInput v-model="nickname" placeholder="请输入昵称" />
					</div>
					<div class="space-y-2">
						<div class="text-sm font-medium text-foreground">头像</div>
						<UiAvatarUpload :preview-url="avatarPreviewUrl || undefined" @change="onAvatarChange" />
					</div>
					<div class="pt-1">
						<UiButton class="h-11 w-full" :disabled="saving" @click="onSave">
							{{ saving ? '保存中…' : '保存修改' }}
						</UiButton>
					</div>
				</div>
			</UiCard>
		</div>
	</BiliLayout>
</template>

<style scoped>
</style>
