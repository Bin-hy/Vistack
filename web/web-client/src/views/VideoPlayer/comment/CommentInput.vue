<script setup lang="ts">
import { computed, ref } from 'vue'
import { UiButton, UiIcon } from '@/components/ui'
import { toast } from '@/components/ui/toast/useToast'
import { useUserStore } from '@/stores/user'
import { createComment, uploadCommentImage } from './api'
import type { CommentItem } from './types'

const props = defineProps<{
  videoId: string
  replyingTo: CommentItem | null
}>()

const emit = defineEmits<{
  (e: 'submitted'): void
  (e: 'cancel-reply'): void
}>()

const userStore = useUserStore()
const content = ref('')
const attachments = ref<{ type: string; file_id: number; url: string }[]>([])
const uploading = ref(false)
const submitting = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

const replyLabel = computed(() => {
  if (!props.replyingTo) return ''
  return `回复 @${props.replyingTo.author?.nickname || '用户'}`
})

function pickImage() {
  fileInput.value?.click()
}

async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0) return
  const file = files[0]
  if (!file) return
  input.value = ''
  if (attachments.value.length >= 9) {
    toast({ title: '最多 9 张图片', type: 'error' })
    return
  }
  uploading.value = true
  try {
    const resp = await uploadCommentImage(file)
    attachments.value.push({ type: 'image', file_id: Number(resp.file_id), url: resp.url })
  } catch (err: any) {
    toast({ title: err?.message ?? '上传失败', type: 'error' })
  } finally {
    uploading.value = false
  }
}

function removeAttachment(index: number) {
  attachments.value.splice(index, 1)
}

async function submit() {
  if (!userStore.isLoggedIn) {
    toast({ title: '请先登录', type: 'error' })
    return
  }
  if (!content.value.trim() && attachments.value.length === 0) {
    toast({ title: '请输入评论内容', type: 'error' })
    return
  }
  submitting.value = true
  try {
    const payload = {
      content: content.value.trim(),
      attachments: attachments.value.map((a) => ({ type: a.type, file_id: a.file_id })),
      parent_id: props.replyingTo ? Number(props.replyingTo.id) : null,
      reply_to_id: props.replyingTo ? Number(props.replyingTo.id) : null,
    }
    await createComment(props.videoId, payload)
    content.value = ''
    attachments.value = []
    emit('submitted')
  } catch (e: any) {
    toast({ title: e?.message ?? '发表失败', type: 'error' })
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="flex gap-3">
    <div
      class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-sm font-semibold text-white"
    >
      <img
        v-if="userStore.currentUser?.avatar_url"
        :src="userStore.currentUser.avatar_url"
        alt="avatar"
        class="h-full w-full object-cover"
      />
      <span v-else>{{ (userStore.displayName || 'U')[0] }}</span>
    </div>

    <div class="min-w-0 flex-1">
      <div v-if="replyingTo" class="mb-1 flex items-center gap-2 text-xs text-primary">
        <span>{{ replyLabel }}</span>
        <button type="button" class="text-muted-foreground hover:text-foreground" @click="emit('cancel-reply')">
          取消
        </button>
      </div>

      <textarea
        v-model="content"
        rows="2"
        class="w-full resize-none rounded-lg border border-border bg-input/80 px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary/50 focus:outline-none focus:ring-2 focus:ring-ring/40"
        placeholder="发一条友善的评论"
      ></textarea>

      <div v-if="attachments.length" class="mt-2 flex flex-wrap gap-2">
        <div v-for="(att, i) in attachments" :key="i" class="relative">
          <img :src="att.url" alt="preview" class="h-16 w-16 rounded-lg object-cover" />
          <button
            type="button"
            class="absolute -right-1.5 -top-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-black/70 text-[10px] text-white"
            @click="removeAttachment(i)"
          >
            ×
          </button>
        </div>
      </div>

      <div class="mt-2 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
            :disabled="uploading"
            @click="pickImage"
          >
            <UiIcon name="upload" :size="16" />
            <span>{{ uploading ? '上传中' : '图片' }}</span>
          </button>
          <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onFileChange" />
        </div>
        <UiButton size="sm" :disabled="submitting" @click="submit">发表评论</UiButton>
      </div>
    </div>
  </div>
</template>
