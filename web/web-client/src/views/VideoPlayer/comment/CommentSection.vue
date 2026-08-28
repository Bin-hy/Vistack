<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { listComments, listReplies } from './api'
import CommentInput from './CommentInput.vue'
import CommentItem from './CommentItem.vue'
import type { CommentItem as CommentItemType } from './types'

const props = defineProps<{ videoId: string }>()

const roots = ref<CommentItemType[]>([])
const total = ref(0)
const nextCursor = ref(0)
const loading = ref(false)
const replyingTo = ref<CommentItemType | null>(null)

const repliesMap = ref<Record<string, CommentItemType[]>>({})
const expanded = ref<Record<string, boolean>>({})
const loadingReplies = ref<Record<string, boolean>>({})

async function load() {
  loading.value = true
  try {
    const resp = await listComments(props.videoId, 0, 20)
    roots.value = resp.comments
    total.value = Number(resp.total)
    nextCursor.value = Number(resp.next_cursor)
  } catch {
    roots.value = []
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  if (loading.value || nextCursor.value <= 0) return
  loading.value = true
  try {
    const resp = await listComments(props.videoId, nextCursor.value, 20)
    roots.value = roots.value.concat(resp.comments)
    nextCursor.value = Number(resp.next_cursor)
  } finally {
    loading.value = false
  }
}

async function toggleReplies(root: CommentItemType) {
  const id = root.id
  if (expanded.value[id]) {
    expanded.value[id] = false
    return
  }
  expanded.value[id] = true
  if (!repliesMap.value[id]) {
    loadingReplies.value[id] = true
    try {
      const resp = await listReplies(id, 0, 50)
      repliesMap.value[id] = resp.comments
    } catch {
      repliesMap.value[id] = []
    } finally {
      loadingReplies.value[id] = false
    }
  }
}

function handleReply(comment: CommentItemType) {
  replyingTo.value = comment
}

function handleSubmitted() {
  replyingTo.value = null
  repliesMap.value = {}
  expanded.value = {}
  load()
}

function handleDeleted() {
  repliesMap.value = {}
  expanded.value = {}
  load()
}

onMounted(load)
</script>

<template>
  <section class="glass rounded-2xl p-4 sm:p-5">
    <h2 class="mb-4 flex items-center gap-2 font-semibold text-foreground">
      评论
      <span class="text-sm font-normal text-muted-foreground">{{ total }}</span>
    </h2>

    <CommentInput
      :video-id="videoId"
      :replying-to="replyingTo"
      @submitted="handleSubmitted"
      @cancel-reply="replyingTo = null"
    />

    <div class="mt-4">
      <div v-if="loading && roots.length === 0" class="space-y-2 py-6">
        <div v-for="i in 3" :key="i" class="h-16 animate-pulse rounded-lg bg-secondary"></div>
      </div>

      <div v-else-if="roots.length === 0" class="py-8 text-center text-sm text-muted-foreground">
        还没有评论，来抢沙发吧～
      </div>

      <div v-else class="divide-y divide-border/60">
        <template v-for="root in roots" :key="root.id">
          <CommentItem :comment="root" @reply="handleReply" @deleted="handleDeleted" />

          <div v-if="root.reply_count > 0 && !root.deleted" class="pb-3 pl-12">
            <button
              v-if="!expanded[root.id]"
              type="button"
              class="text-xs text-primary hover:underline"
              @click="toggleReplies(root)"
            >
              展开 {{ root.reply_count }} 条回复
            </button>
            <template v-else>
              <div v-if="loadingReplies[root.id]" class="py-2 text-xs text-muted-foreground">加载中...</div>
              <div v-else-if="repliesMap[root.id]?.length" class="divide-y divide-border/40">
                <CommentItem
                  v-for="reply in repliesMap[root.id]"
                  :key="reply.id"
                  :comment="reply"
                  is-reply
                  @reply="handleReply"
                  @deleted="handleDeleted"
                />
              </div>
              <div v-else class="py-2 text-xs text-muted-foreground">暂无回复</div>
              <button
                type="button"
                class="text-xs text-muted-foreground hover:text-foreground"
                @click="toggleReplies(root)"
              >
                收起
              </button>
            </template>
          </div>
        </template>
      </div>

      <div v-if="nextCursor > 0" class="mt-4 text-center">
        <button
          type="button"
          class="text-sm text-primary hover:underline"
          :disabled="loading"
          @click="loadMore"
        >
          加载更多
        </button>
      </div>
    </div>
  </section>
</template>
