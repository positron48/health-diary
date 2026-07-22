<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ApiError } from '../api/client'
import { journalApi } from '../api/journal'
import type { HealthEvent } from '../api/types'
import { descriptorFor, entryIdOf } from '../features/events/eventRegistry'
import SourceEntrySheet from '../features/events/SourceEntrySheet.vue'
import StatePanel from '../components/ui/StatePanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useSession } from '../composables/useSession'
import { instantToLocalInput, localInputToUTC } from '../utils/dateTime'

const id = String(useRoute().params.id)
const event = ref<HealthEvent | null>(null)
const loading = ref(true)
const error = ref('')
const saving = ref(false)
const conflict = ref(false)
const sourceId = ref<string | null>(null)
const session = useSession()
const fields = reactive<Record<string, string>>({})
const original = reactive<Record<string, string>>({})
const validation = reactive<Record<string, string>>({})
const locationChoices = [
  { value: 'top_of_head', label: 'Верх головы' },
  { value: 'forehead', label: 'Лоб' },
  { value: 'temple', label: 'Висок' },
  { value: 'occiput_neck', label: 'Затылок/шея' },
  { value: 'neck', label: 'Шея' },
]
const qualityChoices = [
  { value: 'throbbing', label: 'Пульсирующая' },
  { value: 'pressure', label: 'Давящая' },
  { value: 'stabbing', label: 'Колющая' },
  { value: 'burning', label: 'Жгучая' },
]
const selectedLocations = ref<string[]>([])
const selectedQualities = ref<string[]>([])

const descriptor = computed(() => descriptorFor(event.value?.kind || 'note', event.value?.data || event.value?.attributes || {}))
const isPain = computed(() => event.value?.kind === 'pain_observation' || event.value?.kind === 'pain')
const isMed = computed(() => event.value?.kind === 'medication_intake')

function asList(value: unknown) {
  return Array.isArray(value) ? value.map(String) : []
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    event.value = await journalApi.event(id)
    const data = event.value.data || event.value.attributes || {}
    Object.keys(fields).forEach((key) => delete fields[key])
    Object.assign(fields, {
      occurred_at: instantToLocalInput(event.value.occurred_at, session.user.value?.timezone || 'Europe/Moscow'),
      time_precision: event.value.time_precision || 'exact',
      phase: data.phase == null ? '' : String(data.phase),
      intensity: data.intensity == null ? '' : String(data.intensity),
      laterality: data.laterality == null ? '' : String(data.laterality),
      functional_impact: data.functional_impact == null ? '' : String(data.functional_impact),
      name_raw: String(data.name_raw || data.name || ''),
      dose_value: data.dose_value == null ? '' : String(data.dose_value),
      dose_unit: data.dose_unit == null ? '' : String(data.dose_unit),
      effect_rating: data.effect_rating == null ? '' : String(data.effect_rating),
    })
    selectedLocations.value = asList(data.locations)
    selectedQualities.value = asList(data.qualities)
    Object.assign(original, fields)
    original.locations = selectedLocations.value.join(',')
    original.qualities = selectedQualities.value.join(',')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function toggle(list: string[], value: string) {
  const index = list.indexOf(value)
  if (index >= 0) list.splice(index, 1)
  else list.push(value)
}

async function save() {
  if (!event.value) return
  saving.value = true
  conflict.value = false
  Object.keys(validation).forEach((key) => delete validation[key])
  const data: Record<string, unknown> = {}
  const patch: Record<string, unknown> = { revision: event.value.revision }
  const numericKeys = new Set(['intensity', 'functional_impact', 'dose_value', 'effect_rating'])
  for (const [key, val] of Object.entries(fields)) {
    if (val === original[key]) continue
    if (key === 'occurred_at') patch.occurred_at = localInputToUTC(val, session.user.value?.timezone || 'Europe/Moscow')
    else if (key === 'time_precision') patch.time_precision = val
    else if (['phase', 'laterality', 'name_raw', 'dose_unit'].includes(key) || numericKeys.has(key)) {
      data[key] = val === '' ? null : (numericKeys.has(key) ? Number(val) : val)
    }
  }
  const locationsKey = selectedLocations.value.join(',')
  const qualitiesKey = selectedQualities.value.join(',')
  if (isPain.value && locationsKey !== original.locations) data.locations = selectedLocations.value.length ? selectedLocations.value : null
  if (isPain.value && qualitiesKey !== original.qualities) data.qualities = selectedQualities.value.length ? selectedQualities.value : null
  if (Object.keys(data).length) patch.data = data
  try {
    event.value = await journalApi.update(id, patch)
    Object.assign(original, fields)
    original.locations = locationsKey
    original.qualities = qualitiesKey
  } catch (e) {
    if (e instanceof ApiError) {
      Object.assign(validation, e.fields)
      conflict.value = e.isConflict
    }
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}

function openSource() {
  if (!event.value) return
  sourceId.value = entryIdOf(event.value)
}

onMounted(load)
</script>
<template>
  <div class="page">
    <StatePanel v-if="loading" kind="loading"/>
    <StatePanel v-else-if="!event" kind="error" :message="error" @retry="load"/>
    <form v-else class="card stack" @submit.prevent="save">
      <header>
        <p class="eyebrow">Редактирование</p>
        <h1>{{ descriptor.label }}</h1>
      </header>
      <p v-if="error && !conflict" role="alert">{{ error }}</p>
      <div v-if="conflict" class="card" role="alert">
        <strong>Событие изменилось в другом месте.</strong>
        <p>Ваш черновик сохранён в форме.</p>
        <UiButton type="button" variant="secondary" @click="load">Загрузить актуальную версию</UiButton>
      </div>
      <label class="field">Дата и время
        <input v-model="fields.occurred_at" type="datetime-local"/>
        <span class="field-error">{{ validation.occurred_at }}</span>
      </label>
      <label class="field">Точность времени
        <select v-model="fields.time_precision">
          <option value="exact">Точное</option>
          <option value="approximate">Примерное</option>
          <option value="inferred_from_message">Из контекста сообщения</option>
          <option value="date_only">Только дата</option>
        </select>
      </label>
      <template v-if="isPain">
        <label class="field">Фаза
          <select v-model="fields.phase">
            <option value="">Не указано</option>
            <option value="start">Началась</option>
            <option value="update">Наблюдение</option>
            <option value="end">Прошла</option>
          </select>
        </label>
        <label class="field">Интенсивность (0–10)
          <input v-model="fields.intensity" type="number" min="0" max="10" placeholder="Не указано"/>
          <span class="field-error">{{ validation['data.intensity'] || validation.intensity }}</span>
        </label>
        <fieldset class="field">
          <legend>Область</legend>
          <div class="chip-row">
            <button v-for="item in locationChoices" :key="item.value" type="button" class="chip" :class="{ active: selectedLocations.includes(item.value) }" @click="toggle(selectedLocations, item.value)">{{ item.label }}</button>
          </div>
        </fieldset>
        <label class="field">Сторона
          <select v-model="fields.laterality">
            <option value="">Не указано</option>
            <option value="left">Слева</option>
            <option value="right">Справа</option>
            <option value="bilateral">С обеих сторон</option>
            <option value="center">По центру</option>
          </select>
        </label>
        <fieldset class="field">
          <legend>Характер</legend>
          <div class="chip-row">
            <button v-for="item in qualityChoices" :key="item.value" type="button" class="chip" :class="{ active: selectedQualities.includes(item.value) }" @click="toggle(selectedQualities, item.value)">{{ item.label }}</button>
          </div>
        </fieldset>
        <label class="field">Влияние на активность (0–3)
          <input v-model="fields.functional_impact" type="number" min="0" max="3" placeholder="Не указано"/>
        </label>
      </template>
      <template v-else-if="isMed">
        <label class="field">Название
          <input v-model="fields.name_raw" type="text" placeholder="Не указано"/>
        </label>
        <label class="field">Доза
          <input v-model="fields.dose_value" type="number" min="0" step="any" placeholder="Не указано"/>
        </label>
        <label class="field">Единица
          <select v-model="fields.dose_unit">
            <option value="">Не указано</option>
            <option value="мг">мг</option>
            <option value="mg">mg</option>
            <option value="таб">таб</option>
          </select>
        </label>
        <label class="field">Эффект (-2…2)
          <select v-model="fields.effect_rating">
            <option value="">Не указано</option>
            <option value="-2">Сильно хуже</option>
            <option value="-1">Хуже</option>
            <option value="0">Без изменений</option>
            <option value="1">Лучше</option>
            <option value="2">Сильно лучше</option>
          </select>
        </label>
      </template>
      <div class="cluster">
        <UiButton :busy="saving">Сохранить</UiButton>
        <UiButton v-if="entryIdOf(event)" type="button" variant="ghost" @click="openSource">Исходная запись</UiButton>
        <RouterLink :to="`/day/${event.occurred_at.slice(0,10)}`">Отмена</RouterLink>
      </div>
    </form>
    <SourceEntrySheet :entry-id="sourceId" @close="sourceId=null" />
  </div>
</template>
<style scoped>
.chip-row { display:flex; flex-wrap:wrap; gap:var(--s2); }
.chip { min-height:40px; border:1px solid var(--border); border-radius:999px; background:#fff; padding:0 var(--s3); cursor:pointer; }
.chip.active { border-color:var(--primary); background:#e8f0ea; color:var(--primary); font-weight:700; }
fieldset.field { border:0; padding:0; margin:0; }
legend { color:var(--muted); margin-bottom:var(--s2); }
</style>
