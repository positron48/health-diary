<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ApiError } from '../api/client'
import { journalApi } from '../api/journal'
import type { PendingBatch } from '../api/types'
import { useAsyncState } from '../composables/useAsyncState'
import { useToast } from '../composables/useToast'
import EventCard from '../features/events/EventCard.vue'
import StatePanel from '../components/ui/StatePanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiDialog from '../components/ui/UiDialog.vue'
const { data, loading, error, load } = useAsyncState(() => journalApi.pending())
const toast = useToast(), busy = ref(''), rejectTarget = ref<PendingBatch | null>(null), conflict = ref('')
async function transition(batch: PendingBatch, action: 'confirm'|'reject') {
  const index = data.value?.batches.indexOf(batch) ?? -1
  if (index >= 0) data.value!.batches.splice(index, 1)
  busy.value = batch.id; conflict.value = ''
  try { await journalApi[action](batch.id, batch.version); toast.show(action === 'confirm' ? 'Запись подтверждена' : 'Запись отклонена') }
  catch (e) { if (index >= 0) data.value!.batches.splice(index, 0, batch); conflict.value = e instanceof ApiError && e.isConflict ? 'Запись уже изменилась. Загрузите актуальную версию.' : (e as Error).message }
  finally { busy.value = ''; rejectTarget.value = null }
}
onMounted(() => load())
</script>
<template><div class="page stack"><header><p class="eyebrow">Проверка фактов</p><h1>Входящие</h1><p class="muted">Неподтверждённые данные не участвуют в аналитике.</p></header><p v-if="conflict" role="alert" class="card">{{ conflict }} <button class="link" @click="load(true)">Загрузить актуальную версию</button></p><StatePanel v-if="loading" kind="loading" /><StatePanel v-else-if="error" kind="error" :message="error" @retry="load()" /><StatePanel v-else-if="!data?.batches.length" kind="empty" title="Все записи проверены" message="Новые распознанные факты появятся здесь." /><article v-for="batch in data?.batches" v-else :key="batch.id" class="card stack"><div><strong>Telegram</strong><p class="muted">{{ new Date(batch.created_at).toLocaleString('ru-RU') }}</p></div><EventCard v-for="event in batch.events" :key="event.id" :event="event" /><div class="cluster"><UiButton :busy="busy===batch.id" @click="transition(batch,'confirm')">Всё верно</UiButton><UiButton variant="secondary" disabled title="Будет доступно после появления API коррекции">Исправить</UiButton><UiButton variant="ghost" @click="rejectTarget=batch">Отклонить</UiButton></div></article><UiDialog :open="!!rejectTarget" title="Отклонить запись?" @close="rejectTarget=null"><p>Распознанные события не попадут в дневник. Исходная запись сохранится согласно настройкам хранения.</p><div class="cluster"><UiButton variant="danger" :busy="busy===rejectTarget?.id" @click="rejectTarget && transition(rejectTarget,'reject')">Отклонить</UiButton><UiButton variant="secondary" @click="rejectTarget=null">Отмена</UiButton></div></UiDialog></div></template>
