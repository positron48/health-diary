<script setup lang="ts">
import { nextTick, onBeforeUnmount, watch } from 'vue'
const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
let previous: Element | null = null
function keys(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
  if (event.key !== 'Tab') return
  const root = event.currentTarget as HTMLElement, focusable = [...root.querySelectorAll<HTMLElement>('button,a,input,select,textarea,[tabindex]:not([tabindex="-1"])')]
  if (!focusable.length) return
  const first = focusable[0], last = focusable.at(-1)!
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
watch(() => props.open, async (open) => {
  if (open) { previous = document.activeElement; await nextTick(); document.querySelector<HTMLElement>('[data-dialog] button')?.focus() }
  else (previous as HTMLElement | null)?.focus()
})
onBeforeUnmount(() => (previous as HTMLElement | null)?.focus())
</script>
<template><Teleport to="body"><div v-if="open" class="dialog-backdrop" @click.self="$emit('close')"><section data-dialog class="dialog" role="dialog" aria-modal="true" :aria-label="title" @keydown="keys"><header><h2>{{ title }}</h2><button class="icon-button" aria-label="Закрыть" @click="$emit('close')">×</button></header><slot /></section></div></Teleport></template>
