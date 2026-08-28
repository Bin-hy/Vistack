<script setup lang="ts">
import { ref } from 'vue'
import { UiIcon } from '@/components/ui'
import { toast } from '@/components/ui/toast/useToast'
import { useUserStore } from '@/stores/user'
import { formatCount } from '@/lib/format'
import { deleteComment, toggleCommentLike } from './api'
import type { CommentItem } from './types'

const props = withDefaults(
  defineProps<{
    comment: CommentItem
    isReply?: boolean
  }>(),
  { isReply: false },
)

const emit = defineEmits<{
  (e: 'reply', comment: CommentItem): void
  (e: 'deleted', comment: CommentItem): void
}>()

const userStore = useUserStore()
const liked = ref(props.comment.liked)
const likeCount = ref(props.comment.like_count)
const deleting = ref(false)

function isOwn(): boolean {
  return String(userStore.currentUser?.id) === props.comment.author?.id
}

function formatRelative(value: string): string {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const diff = Date.now() - d.getTime()
  if (diff < 60 * 1000) return '刚刚'
  if (diff < 60 * 60 * 1000) return `${Math.floor(diff / (60 * 1000))}分钟前`
  if (diff < 24 * 60 * 60 * 1000) return `${Math.floor(diff / (60 * 60 * 1000))}小时前`
  if (diff < 30 * 24 * 60 * 60 * 1000) return `${Math.floor(diff / (24 * 60 * 60 * 1000))}天前`
  return d.toLocaleDateString('zh-CN')
}

async function handleLike() {
  if (!userStore.isLoggedIn) {
    toast({ title: '请先登录', type: 'error' })
    return
  }
  try {
    const resp = await toggleCommentLike(props.comment.id)
    liked.value = resp.liked
    likeCount.value = resp.like_count
  } catch (e: any) {
    toast({ title: e?.message ?? '操作失败', type: 'error' })
  }
}

function handleReply() {
  if (!userStore.isLoggedIn) {
    toast({ title: '请先登录', type: 'error' })
    return
  }
  emit('reply', props.comment)
}

async function handleDelete() {
  if (deleting.value) return
  deleting.value = true
  try {
    await deleteComment(props.comment.id)
    toast({ title: '已删除', type: 'success' })
    emit('deleted', props.comment)
  } catch (e: any) {
    toast({ title: e?.message ?? '删除失败', type: 'error' })
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div class="flex gap-3 py-3">
    <div
      class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-sm font-semibold text-white"
    >
      <img
        v-if="comment.author?.avatar_url"
        :src="comment.author.avatar_url"
        alt="avatar"
        class="h-full w-full object-cover"
      />
      <span v-else>{{ (comment.author?.nickname || 'U')[0] }}</span>
    </div>
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2 text-xs text-muted-foreground">
        <span class="font-medium text-foreground">{{ comment.author?.nickname || '未知用户' }}</span>
        <span>{{ formatRelative(comment.created_at) }}</span>
      </div>

      <div class="mt-1 text-sm leading-relaxed text-foreground/90">
        <span v-if="comment.deleted || !comment.content" class="italic text-muted-foreground">该评论已删除</span>
        <template v-else>
          <span v-if="isReply && comment.reply_to_author" class="mr-1 text-primary">
            回复 @{{ comment.reply_to_author.nickname }}：
          </span>
          <span>{{ comment.content }}</span>
        </template>
      </div>

      <div v-if="comment.attachments?.length" class="mt-2 flex flex-wrap gap-2">
        <img
          v-for="(att, i) in comment.attachments"
          :key="i"
          :src="att.url"
          alt="attachment"
          class="h-20 w-20 rounded-lg object-cover"
          loading="lazy"
        />
      </div>

      <div class="mt-2 flex items-center gap-4 text-xs text-muted-foreground">
        <button
          class="flex items-center gap-1 hover:text-primary"
          :class="liked ? 'text-primary' : ''"
          type="button"
          @click="handleLike"
        >
          <UiIcon name="thumbs-up" :size="14" :class="liked ? 'fill-current' : ''" />
          <span v-if="likeCount > 0">{{ formatCount(likeCount) }}</span>
        </button>
        <button class="hover:text-primary" type="button" @click="handleReply">回复</button>
        <button
          v-if="isOwn()"
          class="hover:text-destructive"
          type="button"
          :disabled="deleting"
          @click="handleDelete"
        >
          删除
        </button>
      </div>
    </div>
  </div>
</template>
