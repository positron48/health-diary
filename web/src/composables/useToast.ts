import { readonly, ref } from 'vue'
export interface ToastItem { id: number; message: string; action?: string; onAction?: () => void }
const items = ref<ToastItem[]>([])
let id = 0
export function useToast() {
  function show(message: string, action?: string, onAction?: () => void) {
    const item = { id: ++id, message, action, onAction }; items.value.push(item)
    window.setTimeout(() => dismiss(item.id), 6000)
  }
  function dismiss(itemId: number) { items.value = items.value.filter((item) => item.id !== itemId) }
  return { items: readonly(items), show, dismiss }
}
