<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ApiError } from '../api/client'
import { journalApi } from '../api/journal'
import type { InboxResponse, PendingBatch, ProcessingEntry } from '../api/types'
import { useToast } from '../composables/useToast'
import EventCard from '../features/events/EventCard.vue'
import SourceEntrySheet from '../features/events/SourceEntrySheet.vue'
import StatePanel from '../components/ui/StatePanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiDialog from '../components/ui/UiDialog.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const data = ref<InboxResponse | null>(null)
const loading = ref(true)
const error = ref('')
const busy = ref('')
const rejectTarget = ref<PendingBatch | null>(null)
const conflict = ref('')
const sourceId = ref<string | null>(null)
const sourceSheet = ref<{ load: () => Promise<void> } | null>(null)
const optimistic = ref<ProcessingEntry[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

watch(sourceId, (id) => { if (id) sourceSheet.value?.load() })

const processing = computed(() => {
  const server = data.value?.processing || []
  const known = new Set(server.map((item) => item.id))
  const extras = optimistic.value.filter((item) => !known.has(item.id))
  return [...extras, ...server]
})
const batches = computed(() => data.value?.batches || [])
const isEmpty = computed(() => !processing.value.length && !batches.value.length)

function processingLabel(status: string) {
  switch (status) {
    case 'queued': return 'В очереди на распознавание'
    case 'processing': return 'Распознаём запись…'
    case 'failed': return 'Не удалось распознать автоматически'
    default: return 'В обработке'
  }
}

async function load(quiet = false) {
  if (!quiet) {
    loading.value = !data.value
    error.value = ''
  }
  try {
    const next = await journalApi.inbox()
    data.value = next
    const serverIds = new Set(next.processing.map((item) => item.id))
    const batchEntryIds = new Set(next.batches.map((batch) => batch.entry_id).filter(Boolean))
    optimistic.value = optimistic.value.filter((item) => !serverIds.has(item.id) && !batchEntryIds.has(item.id))
    if (typeof route.query.entry === 'string' && route.query.entry) {
      const entryID = route.query.entry
      if (batchEntryIds.has(entryID) || serverIds.has(entryID)) {
        const query = { ...route.query }
        delete query.entry
        router.replace({ path: '/pending', query })
      }
    }
  } catch (e) {
    if (!quiet) error.value = e instanceof ApiError ? e.message : 'Не удалось загрузить входящие.'
  } finally {
    loading.value = false
  }
}

function seedOptimistic() {
  const entryID = typeof route.query.entry === 'string' ? route.query.entry : ''
  if (!entryID) return
  if (optimistic.value.some((item) => item.id === entryID)) return
  if (data.value?.processing.some((item) => item.id === entryID)) return
  if (data.value?.batches.some((batch) => batch.entry_id === entryID)) return
  optimistic.value.unshift({
    id: entryID,
    source_type: 'web',
    source_sent_at: new Date().toISOString(),
    processing_status: 'queued',
  })
}

async function transition(batch: PendingBatch, action: 'confirm' | 'reject') {
  const index = data.value?.batches.indexOf(batch) ?? -1
  if (index >= 0) data.value!.batches.splice(index, 1)
  busy.value = batch.id
  conflict.value = ''
  try {
    await journalApi[action](batch.id, batch.version)
    toast.show(action === 'confirm' ? 'Запись подтверждена' : 'Запись отклонена')
  } catch (e) {
    if (index >= 0) data.value!.batches.splice(index, 0, batch)
    conflict.value = e instanceof ApiError && e.isConflict
      ? 'Запись уже изменилась. Загрузите актуальную версию.'
      : (e as Error).message
  } finally {
    busy.value = ''
    rejectTarget.value = null
  }
}

function openSource(entryId?: string | null) {
  sourceId.value = entryId || null
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    if (document.visibilityState === 'visible') load(true)
  }, 2000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function onVisibility() {
  if (document.visibilityState === 'visible') {
    load(true)
    startPolling()
  } else {
    stopPolling()
  }
}

onMounted(async () => {
  seedOptimistic()
  await load()
  seedOptimistic()
  startPolling()
  document.addEventListener('visibilitychange', onVisibility)
})
onUnmounted(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>
<template>
  <div class="page stack">
    <header>
      <p class="eyebrow">Проверка фактов</p>
      <h1>Входящие</h1>
      <p class="muted">Неподтверждённые данные не участвуют в аналитике.</p>
    </header>
    <p v-if="conflict" role="alert" class="card">{{ conflict }} <button class="link" @click="load()">Загрузить актуальную версию</button></p>
    <StatePanel v-if="loading" kind="loading" />
    <StatePanel v-else-if="error" kind="error" :message="error" @retry="load()" />
    <StatePanel v-else-if="isEmpty" kind="empty" title="Все записи проверены" message="Новые распознанные факты появятся здесь." />
    <template v-else>
      <section v-if="processing.length" class="stack">
        <h2 class="section-title">В обработке</h2>
        <article v-for="entry in processing" :key="entry.id" class="card stack processing-card" :class="{ failed: entry.processing_status === 'failed' }">
          <div class="cluster processing-head">
            <strong>{{ entry.source_type === 'web' ? 'Веб' : 'Telegram' }}</strong>
            <span v-if="entry.processing_status !== 'failed'" class="spinner" aria-hidden="true" />
          </div>
          <p class="muted">{{ new Date(entry.source_sent_at).toLocaleString('ru-RU') }}</p>
          <p>{{ processingLabel(entry.processing_status) }}</p>
          <div class="cluster">
            <UiButton variant="ghost" @click="openSource(entry.id)">Показать исходную запись</UiButton>
          </div>
        </article>
      </section>
      <section v-if="batches.length" class="stack">
        <h2 v-if="processing.length" class="section-title">На проверке</h2>
        <article v-for="batch in batches" :key="batch.id" class="card stack">
          <div>
            <strong>{{ batch.source_type === 'web' ? 'Веб' : 'Telegram' }}</strong>
            <p class="muted">{{ new Date(batch.message_at || batch.created_at).toLocaleString('ru-RU') }}</p>
          </div>
          <EventCard v-for="event in batch.events" :key="event.id" :event="event" />
          <div class="cluster">
            <UiButton :busy="busy===batch.id" @click="transition(batch,'confirm')">Всё верно</UiButton>
            <UiButton variant="secondary" disabled title="Будет доступно после появления API коррекции">Исправить</UiButton>
            <UiButton variant="ghost" @click="rejectTarget=batch">Отклонить</UiButton>
            <UiButton v-if="batch.entry_id" variant="ghost" @click="openSource(batch.entry_id)">Показать исходную запись</UiButton>
          </div>
        </article>
      </section>
    </template>
    <UiDialog :open="!!rejectTarget" title="Отклонить запись?" @close="rejectTarget=null">
      <p>Распознанные события не попадут в дневник. Исходная запись сохранится согласно настройкам хранения.</p>
      <div class="cluster">
        <UiButton variant="danger" :busy="busy===rejectTarget?.id" @click="rejectTarget && transition(rejectTarget,'reject')">Отклонить</UiButton>
        <UiButton variant="secondary" @click="rejectTarget=null">Отмена</UiButton>
      </div>
    </UiDialog>
    <SourceEntrySheet ref="sourceSheet" :entry-id="sourceId" @close="sourceId=null" />
  </div>
</template>
<style scoped>
.section-title { font-size: 1rem; margin: 0; }
.processing-head { justify-content: space-between; }
.processing-card.failed { border-color: #c45c5c; }
.spinner {
  width: 1rem; height: 1rem; border-radius: 50%;
  border: 2px solid var(--border); border-top-color: var(--primary);
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
