import { reactive } from 'vue'

export type ToastType = 'success' | 'error' | 'info'

export interface ToastItem {
  id: number
  title: string
  description?: string
  type: ToastType
}

interface ToastState {
  items: ToastItem[]
}

const state = reactive<ToastState>({ items: [] })
let seed = 0

export function toast(opts: { title: string; description?: string; type?: ToastType; duration?: number }) {
  const id = ++seed
  state.items.push({
    id,
    title: opts.title,
    description: opts.description,
    type: opts.type ?? 'info',
  })
  const duration = opts.duration ?? 3000
  setTimeout(() => dismiss(id), duration)
}

export function dismiss(id: number) {
  const idx = state.items.findIndex((t) => t.id === id)
  if (idx !== -1) state.items.splice(idx, 1)
}

export function useToast() {
  return { items: state.items, toast, dismiss }
}
