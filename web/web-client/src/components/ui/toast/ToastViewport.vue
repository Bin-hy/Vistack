<script setup lang="ts">
import { useToast, type ToastType } from './useToast'
import { UiIcon } from '@/components/ui'

const { items, dismiss } = useToast()

function iconFor(type: ToastType): string {
  if (type === 'success') return 'check-circle'
  if (type === 'error') return 'alert-circle'
  return 'info'
}

function colorFor(type: ToastType): string {
  if (type === 'success') return 'text-emerald-400'
  if (type === 'error') return 'text-destructive'
  return 'text-primary'
}
</script>

<template>
  <div class="fixed bottom-4 right-4 z-[100] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2">
    <TransitionGroup name="toast">
      <div
        v-for="item in items"
        :key="item.id"
        class="glass-strong flex items-start gap-2 rounded-lg p-3 shadow-xl shadow-black/30"
      >
        <UiIcon :name="iconFor(item.type)" :size="18" :class="colorFor(item.type) + ' mt-0.5 shrink-0'" />
        <div class="min-w-0 flex-1">
          <div class="text-sm font-medium text-foreground">{{ item.title }}</div>
          <div v-if="item.description" class="mt-0.5 text-xs text-muted-foreground">{{ item.description }}</div>
        </div>
        <button class="shrink-0 text-muted-foreground hover:text-foreground" @click="dismiss(item.id)">
          <UiIcon name="x" :size="14" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.2s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(16px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(16px);
}
</style>
