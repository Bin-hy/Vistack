<script setup lang="ts">
import { useConfirm } from './useConfirm'
import { UiButton } from '@/components/ui'

const { state, resolveConfirm } = useConfirm()
</script>

<template>
  <Teleport to="body">
    <div v-if="state.open" class="fixed inset-0 z-[90] flex items-center justify-center p-4">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="resolveConfirm(false)"></div>
      <div class="glass-strong relative w-full max-w-sm rounded-xl p-6 shadow-2xl shadow-black/50 animate-scale-in">
        <h2 class="text-base font-semibold text-foreground">{{ state.title }}</h2>
        <p v-if="state.description" class="mt-2 text-sm text-muted-foreground">{{ state.description }}</p>
        <div class="mt-6 flex justify-end gap-2">
          <UiButton variant="outline" size="sm" @click="resolveConfirm(false)">{{ state.cancelText }}</UiButton>
          <UiButton
            :variant="state.danger ? 'destructive' : 'default'"
            size="sm"
            @click="resolveConfirm(true)"
          >
            {{ state.confirmText }}
          </UiButton>
        </div>
      </div>
    </div>
  </Teleport>
</template>
