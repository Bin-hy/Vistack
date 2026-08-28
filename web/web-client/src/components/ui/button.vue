<script setup lang="ts">
type Variant = 'default' | 'secondary' | 'destructive' | 'outline' | 'ghost' | 'link'
type Size = 'sm' | 'md' | 'lg'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: Size
  disabled?: boolean
  class?: string
}>(), {
  variant: 'default',
  size: 'md',
  disabled: false,
  class: '',
})

const base =
  'inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-md text-sm font-medium transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50'

const variantClasses: Record<Variant, string> = {
  default:
    'bg-gradient-to-r from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-white shadow-lg shadow-black/20 hover:shadow-glow-sm hover:brightness-110',
  secondary:
    'bg-secondary text-secondary-foreground shadow-sm hover:bg-secondary/80',
  destructive:
    'bg-destructive text-destructive-foreground shadow hover:bg-destructive/90',
  outline:
    'glass text-foreground hover:bg-accent hover:border-primary/30',
  ghost: 'hover:bg-accent hover:text-accent-foreground',
  link: 'text-primary underline-offset-4 hover:underline',
}

const sizeClasses: Record<Size, string> = {
  sm: 'h-9 px-3',
  md: 'h-10 px-4 py-2',
  lg: 'h-11 px-6',
}
</script>

<template>
  <button
    :disabled="props.disabled"
    :class="[base, variantClasses[props.variant], sizeClasses[props.size], props.class]"
  >
    <slot />
  </button>
</template>
