<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { contextApi } from '../../api/context'
import { ApiError } from '../../api/client'
import type { ContextPeriod, ContextPeriodsResponse, ContextPeriodType, PlaceCandidate } from '../../api/types'
import { useSession } from '../../composables/useSession'
import { useToast } from '../../composables/useToast'
import { userDate } from '../../utils/userDay'
import UiButton from '../../components/ui/UiButton.vue'
import UiDialog from '../../components/ui/UiDialog.vue'

const session = useSession()
const toast = useToast()
const fallbackDate = userDate(
  new Date(),
  session.user.value?.timezone || 'Europe/Moscow',
  session.user.value?.settings?.day_start_time,
)
const state = ref<ContextPeriodsResponse | null>(null)
const loading = ref(true)
const searchBusy = ref(false)
const saveBusy = ref(false)
const error = ref('')
const fieldErrors = ref<Record<string, string>>({})
const query = ref('')
const hits = ref<PlaceCandidate[]>([])
const selected = ref<PlaceCandidate | null>(null)
const startedOn = ref(session.user.value?.current_local_date || fallbackDate)
const endedOn = ref('')
const periodType = ref<ContextPeriodType>('temporary_stay')
const returnDialog = ref(false)
const cancelTarget = ref<ContextPeriod | null>(null)

const activeCopy = computed(() => {
  if (!state.value?.active) return 'Не определён'
  return state.value.active.source === 'home'
    ? `${state.value.active.place_label} · домашний город`
    : `${state.value.active.place_label} · ${periodTypeLabel(state.value.active.period_type)}`
})

async function load(preserveError = false) {
  if (!preserveError) error.value = ''
  try {
    state.value = await contextApi.list()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function search() {
  if (query.value.trim().length < 2) {
    fieldErrors.value = { query: 'Введите хотя бы две буквы' }
    return
  }
  searchBusy.value = true
  fieldErrors.value = {}
  error.value = ''
  try {
    const response = await contextApi.searchPlaces(query.value.trim())
    hits.value = response.places || []
    if (!hits.value.length) error.value = 'Город не найден. Уточните название.'
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    searchBusy.value = false
  }
}

function choose(place: PlaceCandidate) {
  selected.value = place
  query.value = place.label
  hits.value = []
  fieldErrors.value = {}
}

async function save() {
  fieldErrors.value = {}
  error.value = ''
  if (!selected.value) fieldErrors.value.place_id = 'Сначала найдите и выберите город'
  if (!startedOn.value) fieldErrors.value.started_on = 'Укажите дату начала'
  if (endedOn.value && endedOn.value < startedOn.value) fieldErrors.value.ended_on = 'Дата окончания не может быть раньше начала'
  if (Object.keys(fieldErrors.value).length || !selected.value) return
  saveBusy.value = true
  try {
    await contextApi.createPeriod({
      ...selected.value,
      place_label: selected.value.label,
      period_type: periodType.value,
      started_on: startedOn.value,
      ...(endedOn.value ? { ended_on: endedOn.value } : {}),
    })
    selected.value = null
    query.value = ''
    endedOn.value = ''
    await load(true)
    toast.show('Местоположение сохранено')
  } catch (e) {
    const api = e as ApiError
    fieldErrors.value = api.fields || {}
    error.value = api.code === 'context_period_conflict'
      ? 'На эти даты уже задан другой город. Уберите конфликтующий период или выберите другую дату.'
      : api.message
    await load(true)
  } finally {
    saveBusy.value = false
  }
}

async function confirmReturnHome() {
  saveBusy.value = true
  error.value = ''
  try {
    await contextApi.returnHome(startedOn.value)
    returnDialog.value = false
    await load()
    toast.show('Домашний город действует с выбранной даты')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saveBusy.value = false
  }
}

function openReturnHome() {
  if (!startedOn.value) {
    fieldErrors.value = { ...fieldErrors.value, started_on: 'Укажите дату возвращения' }
    return
  }
  returnDialog.value = true
}

async function cancelPeriod() {
  if (!cancelTarget.value) return
  saveBusy.value = true
  error.value = ''
  try {
    await contextApi.cancelPeriod(cancelTarget.value)
    cancelTarget.value = null
    await load()
    toast.show('Период убран')
  } catch (e) {
    const api = e as ApiError
    error.value = api.isConflict ? 'Период уже изменился. История обновлена.' : api.message
    cancelTarget.value = null
    await load(true)
  } finally {
    saveBusy.value = false
  }
}

function periodTypeLabel(value?: ContextPeriodType) {
  return ({ vacation: 'отпуск', trip: 'поездка', temporary_stay: 'временное пребывание', relocation: 'переезд', other: 'другое' } as Record<string, string>)[value || ''] || 'период'
}

function formatDate(value: string) {
  return new Date(`${value}T12:00:00`).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
}

onMounted(load)
</script>

<template>
  <section class="card stack" aria-labelledby="location-title">
    <div>
      <h2 id="location-title">Местоположение</h2>
      <p class="muted">Только город — без GPS и адреса. Дата включается в выбранный период.</p>
    </div>
    <p v-if="loading" class="muted">Загружаю текущее местоположение…</p>
    <div v-else class="current-location">
      <span class="muted">Сейчас</span>
      <strong>{{ activeCopy }}</strong>
    </div>
    <p v-if="error" class="field-error" role="alert">{{ error }}</p>

    <form class="stack" @submit.prevent="save">
      <label class="field">
        Город
        <input
          v-model="query"
          data-testid="location-query"
          autocomplete="off"
          placeholder="Например, Липецк"
          @input="selected = null"
          @keyup.enter.prevent="search"
        >
        <small v-if="selected" class="muted">Выбран: {{ selected.label }}<template v-if="selected.region">, {{ selected.region }}</template></small>
        <small v-if="fieldErrors.query || fieldErrors.place_id" class="field-error">{{ fieldErrors.query || fieldErrors.place_id }}</small>
      </label>
      <UiButton data-testid="location-search" type="button" variant="secondary" :busy="searchBusy" @click="search">Найти город</UiButton>
      <ul v-if="hits.length" class="place-hits" aria-label="Найденные города">
        <li v-for="place in hits" :key="place.provider_place_id">
          <button data-testid="location-hit" type="button" class="button button--secondary place-hit" @click="choose(place)">
            <strong>{{ place.label }}</strong><span v-if="place.region">, {{ place.region }}</span>
          </button>
        </li>
      </ul>
      <div class="date-grid">
        <label class="field">
          С какого дня
          <input v-model="startedOn" data-testid="location-start" type="date">
          <small v-if="fieldErrors.started_on" class="field-error">{{ fieldErrors.started_on }}</small>
        </label>
        <label class="field">
          По какой день
          <input v-model="endedOn" type="date" :min="startedOn">
          <small class="muted">Оставьте пустым — до следующей смены.</small>
          <small v-if="fieldErrors.ended_on" class="field-error">{{ fieldErrors.ended_on }}</small>
        </label>
      </div>
      <label class="field">
        Тип периода
        <select v-model="periodType">
          <option value="temporary_stay">Временное пребывание</option>
          <option value="trip">Поездка</option>
          <option value="vacation">Отпуск</option>
          <option value="relocation">Переезд</option>
          <option value="other">Другое</option>
        </select>
      </label>
      <div class="cluster">
        <UiButton type="submit" :busy="saveBusy">Сохранить город</UiButton>
        <UiButton data-testid="return-home" type="button" variant="ghost" :busy="saveBusy" @click="openReturnHome">Вернулся домой</UiButton>
      </div>
    </form>

    <div v-if="state?.periods.length" class="stack history">
      <h3>История местоположений</h3>
      <article v-for="period in state.periods" :key="period.id" class="period-row">
        <div>
          <strong>{{ period.place_label }}</strong>
          <p class="muted">{{ periodTypeLabel(period.period_type) }} · {{ formatDate(period.started_on) }} — {{ period.ended_on ? formatDate(period.ended_on) : 'до следующей смены' }}</p>
        </div>
        <button type="button" class="link danger-text" @click="cancelTarget = period">Убрать</button>
      </article>
    </div>

    <UiDialog :open="returnDialog" title="Вернуться в домашний город?" @close="returnDialog = false">
      <p>Текущий период завершится накануне. Домашний город будет действовать с {{ formatDate(startedOn) }}.</p>
      <div class="cluster">
        <UiButton data-testid="confirm-return-home" :busy="saveBusy" @click="confirmReturnHome">Подтвердить</UiButton>
        <UiButton variant="secondary" @click="returnDialog = false">Отмена</UiButton>
      </div>
    </UiDialog>
    <UiDialog :open="Boolean(cancelTarget)" title="Убрать период?" @close="cancelTarget = null">
      <p>Город исчезнет из календарного контекста и погодных сопоставлений за этот период.</p>
      <div class="cluster">
        <UiButton variant="danger" :busy="saveBusy" @click="cancelPeriod">Убрать</UiButton>
        <UiButton variant="secondary" @click="cancelTarget = null">Отмена</UiButton>
      </div>
    </UiDialog>
  </section>
</template>

<style scoped>
.current-location { display: grid; gap: var(--s1); padding: var(--s3) var(--s4); border-radius: var(--radius-control); background: #f0f5f1; }
.date-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--s3); }
.place-hits { list-style: none; padding: 0; margin: 0; display: grid; gap: var(--s2); }
.place-hit { width: 100%; text-align: left; }
.history { border-top: 1px solid var(--border); padding-top: var(--s4); }
.period-row { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s3); padding: var(--s3) 0; border-bottom: 1px solid var(--border); }
.period-row p { margin: var(--s1) 0 0; }
@media (max-width: 560px) { .date-grid { grid-template-columns: 1fr; } }
</style>
