<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, post, del } from '@/lib/http'
import { toast } from '@/components/ui/toast/useToast'
import BiliLayout from '@/layouts/BiliLayout.vue'
import { UiInput, UiButton } from '@/components/ui'

interface SensitiveWord {
	id: number
	word: string
	created_at: string
}

const words = ref<SensitiveWord[]>([])
const newWord = ref('')
const loading = ref(false)

async function loadWords() {
	loading.value = true
	try {
		const resp = await get<{ words: SensitiveWord[] }>('/admin/sensitive-words')
		words.value = resp.words || []
	} catch (e: any) {
		toast({ title: e?.message ?? '加载失败，请先登录', type: 'error' })
	} finally {
		loading.value = false
	}
}

async function addWord() {
	const w = newWord.value.trim()
	if (!w) return
	try {
		await post('/admin/sensitive-words', { word: w })
		toast({ title: '已添加', type: 'success' })
		newWord.value = ''
		await loadWords()
	} catch (e: any) {
		toast({ title: e?.message ?? '添加失败（可能已存在）', type: 'error' })
	}
}

async function removeWord(id: number) {
	try {
		await del(`/admin/sensitive-words/${id}`)
		toast({ title: '已删除', type: 'success' })
		await loadWords()
	} catch (e: any) {
		toast({ title: e?.message ?? '删除失败', type: 'error' })
	}
}

onMounted(loadWords)
</script>

<template>
	<BiliLayout>
		<div class="space-y-4">
			<div class="glass rounded-xl p-6">
				<h1 class="text-xl font-semibold">违禁词管理</h1>
				<p class="mt-1 text-sm text-muted-foreground">用于弹幕敏感词过滤（AC 自动机，增删后实时生效）。</p>
			</div>

			<div class="glass rounded-xl p-6">
				<div class="flex gap-2">
					<UiInput v-model="newWord" class="flex-1" placeholder="输入要添加的违禁词" @keyup.enter="addWord" />
					<UiButton @click="addWord">添加</UiButton>
				</div>

				<div class="mt-4">
					<div v-if="loading" class="py-8 text-center text-sm text-muted-foreground">加载中…</div>
					<div v-else-if="words.length === 0" class="py-8 text-center text-sm text-muted-foreground">暂无违禁词</div>
					<ul v-else class="divide-y divide-border">
						<li v-for="w in words" :key="w.id" class="flex items-center justify-between py-2.5">
							<div class="flex min-w-0 items-center gap-2">
								<span class="font-medium">{{ w.word }}</span>
								<span class="text-xs text-muted-foreground">{{ w.created_at }}</span>
							</div>
							<UiButton variant="destructive" size="sm" @click="removeWord(w.id)">删除</UiButton>
						</li>
					</ul>
				</div>
			</div>
		</div>
	</BiliLayout>
</template>
