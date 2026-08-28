import { reactive } from 'vue'

export interface ConfirmState {
  open: boolean
  title: string
  description?: string
  danger?: boolean
  confirmText: string
  cancelText: string
}

const state = reactive<ConfirmState>({
  open: false,
  title: '',
  description: '',
  danger: false,
  confirmText: '确认',
  cancelText: '取消',
})

let resolver: ((value: boolean) => void) | null = null

export function confirm(opts: {
  title: string
  description?: string
  danger?: boolean
  confirmText?: string
  cancelText?: string
}): Promise<boolean> {
  return new Promise((resolve) => {
    state.title = opts.title
    state.description = opts.description
    state.danger = opts.danger ?? false
    state.confirmText = opts.confirmText ?? '确认'
    state.cancelText = opts.cancelText ?? '取消'
    resolver = resolve
    state.open = true
  })
}

export function resolveConfirm(value: boolean) {
  if (resolver) {
    resolver(value)
    resolver = null
  }
  state.open = false
}

export function useConfirm() {
  return { state, confirm, resolveConfirm }
}
