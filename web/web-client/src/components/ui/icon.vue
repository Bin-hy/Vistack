<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import {
  Bell,
  Newspaper,
  Star,
  History,
  Menu,
  Search,
  ThumbsUp,
  Bookmark,
  Share2,
  Settings,
  X,
  ChevronLeft,
  ChevronRight,
  Play,
  Pause,
  Volume2,
  VolumeX,
  Maximize,
  Loader2,
  Upload,
  Trash2,
  Pencil,
  Eye,
  EyeOff,
  User,
  LogOut,
  Plus,
  CheckCircle2,
  AlertCircle,
  Info,
} from 'lucide-vue-next'

const props = withDefaults(defineProps<{
  name: string
  size?: number | string
  class?: string
}>(), {
  size: 20,
  class: '',
})

const iconMap: Record<string, Component> = {
  bell: Bell,
  newspaper: Newspaper,
  star: Star,
  history: History,
  menu: Menu,
  search: Search,
  'thumbs-up': ThumbsUp,
  bookmark: Bookmark,
  share: Share2,
  settings: Settings,
  x: X,
  'chevron-left': ChevronLeft,
  'chevron-right': ChevronRight,
  play: Play,
  pause: Pause,
  volume: Volume2,
  'volume-x': VolumeX,
  maximize: Maximize,
  loader: Loader2,
  upload: Upload,
  trash: Trash2,
  pencil: Pencil,
  eye: Eye,
  'eye-off': EyeOff,
  user: User,
  logout: LogOut,
  plus: Plus,
  'check-circle': CheckCircle2,
  'alert-circle': AlertCircle,
  info: Info,
}

const IconComponent = computed<Component | null>(() => iconMap[props.name] ?? null)
</script>

<template>
  <component
    :is="IconComponent"
    v-if="IconComponent"
    :size="props.size"
    :class="props.class"
  />
</template>
