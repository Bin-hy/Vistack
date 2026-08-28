<script setup lang="ts">
import ToastViewport from '@/components/ui/toast/ToastViewport.vue'
import ConfirmDialog from '@/components/ui/dialog/ConfirmDialog.vue'
</script>

<template>
  <router-view />
  <ToastViewport />
  <ConfirmDialog />
</template>
