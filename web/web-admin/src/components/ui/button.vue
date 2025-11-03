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
  'inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 disabled:pointer-events-none disabled:opacity-50'

const variantClasses: Record<Variant, string> = {
  default: 'bg-primary text-primary-foreground shadow hover:bg-primary/90',
  secondary:
    'bg-secondary text-secondary-foreground shadow-sm hover:bg-secondary/80',
  destructive:
    'bg-destructive text-destructive-foreground shadow hover:bg-destructive/90',
  outline:
    'border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground',
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