<script setup lang="ts">
import { ref, watch } from 'vue'
import { journalApi } from '../../api/journal'
import type { SourceEntry } from '../../api/types'
import UiButton from '../../components/ui/UiButton.vue'
import UiDialog from '../../components/ui/UiDialog.vue'

const props = defineProps<{ entryId: string | null }>()
const emit = defineEmits<{ close: [] }>()
const loading = ref(false)
const error = ref('')
const entry = ref<SourceEntry | null>(null)

async function load() {
  if (!props.entryId) return
  loading.value = true
  error.value = ''
  entry.value = null
  try {
    entry.value = await journalApi.source(props.entryId)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

watch(() => props.entryId, (id) => { if (id) void load() })
</script>
<template>
  <UiDialog :open="!!entryId" title="Исходная запись" @close="emit('close')">
    <div class="stack">
      <p v-if="loading" class="muted">Загрузка…</p>
      <p v-else-if="error" role="alert">{{ error }}</p>
      <template v-else-if="entry">
        <p class="muted">{{ entry.source_type === 'telegram_text' ? 'Telegram' : entry.source_type }} · {{ new Date(entry.source_sent_at).toLocaleString('ru-RU') }}</p>
        <pre class="source-text">{{ entry.text }}</pre>
      </template>
      <div class="cluster">
        <UiButton v-if="error" variant="secondary" @click="load">Повторить</UiButton>
        <UiButton variant="secondary" @click="emit('close')">Закрыть</UiButton>
      </div>
    </div>
  </UiDialog>
</template>
<style scoped>
.source-text {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  padding: var(--s3);
  border-radius: var(--radius-control);
  background: #f0f4f1;
  border: 1px solid var(--border);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: .9rem;
}
</style>
