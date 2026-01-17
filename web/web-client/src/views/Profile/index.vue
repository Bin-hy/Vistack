<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useUserStore } from '@/stores/user'
import BiliLayout from '@/layouts/BiliLayout.vue'
import { UiButton, UiCard, UiInput, UiAvatarUpload } from '@/components/ui'

const userStore = useUserStore()
const nickname = ref('')
const avatarFile = ref<File | null>(null)
const saving = ref(false)

const username = computed(() => userStore.currentUser?.username ?? '')

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
		alert('请先登录')
		return
	}
	saving.value = true
	try {
		await userStore.updateProfileWithAvatar({
			nickname: nickname.value || undefined,
			avatarFile: avatarFile.value,
		})
		alert('资料已更新')
		avatarFile.value = null
	} catch (e: any) {
		alert(e?.message ?? '更新失败，请稍后重试')
	} finally {
		saving.value = false
	}
}
</script>

<template>
	<BiliLayout>
		<UiCard class="max-w-xl p-6">
			<h1 class="text-xl font-semibold mb-4">个人资料</h1>
			<div class="space-y-4">
				<div>
					<div class="text-xs text-gray-500 mb-1">用户名</div>
					<div class="text-sm">{{ username }}</div>
				</div>
				<div class="space-y-1">
					<label class="block text-sm text-gray-600">昵称</label>
					<UiInput v-model="nickname" placeholder="请输入昵称" />
				</div>
				<div class="space-y-2">
					<div class="text-sm text-gray-600">头像</div>
					<UiAvatarUpload :preview-url="avatarPreviewUrl || undefined" @change="onAvatarChange" />
				</div>
				<div>
					<UiButton class="w-full h-11" :disabled="saving" @click="onSave">
						{{ saving ? '保存中…' : '保存修改' }}
					</UiButton>
				</div>
			</div>
		</UiCard>
	</BiliLayout>
</template>

<style scoped>
</style>
