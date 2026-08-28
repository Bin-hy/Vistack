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
  ChevronDown,
  Play,
  Pause,
  PlayCircle,
  Volume2,
  Volume1,
  VolumeX,
  Maximize,
  Minimize,
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
  Home,
  Compass,
  Flame,
  TrendingUp,
  Heart,
  Clock,
  Tv,
  Film,
  Sparkles,
  Gauge,
  Send,
  MessageSquare,
} from 'lucide-vue-next'

const props = withDefaults(defineProps<{
  name: string
  size?: number | string
  class?: string
  strokeWidth?: number | string
}>(), {
  size: 20,
  class: '',
  strokeWidth: 2,
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
  'chevron-down': ChevronDown,
  play: Play,
  pause: Pause,
  'play-circle': PlayCircle,
  volume: Volume2,
  'volume-low': Volume1,
  'volume-x': VolumeX,
  maximize: Maximize,
  minimize: Minimize,
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
  home: Home,
  compass: Compass,
  flame: Flame,
  trending: TrendingUp,
  heart: Heart,
  clock: Clock,
  tv: Tv,
  film: Film,
  sparkles: Sparkles,
  gauge: Gauge,
  send: Send,
  'message-square': MessageSquare,
}

const IconComponent = computed<Component | null>(() => iconMap[props.name] ?? null)
</script>

<template>
  <component
    :is="IconComponent"
    v-if="IconComponent"
    :size="props.size"
    :class="props.class"
    :stroke-width="props.strokeWidth"
    :aria-hidden="true"
  />
</template>
