<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Activity, Bed, CloudSun, HeartPulse, MapPin, Pill, Plus, Smile, ChevronLeft, ChevronRight } from '@lucide/vue'
import { calendarApi, type CalendarLayer } from '../api/calendar'
import { journalApi } from '../api/journal'
import type { CalendarDay } from '../api/types'
import { useAsyncState } from '../composables/useAsyncState'
import { useSession } from '../composables/useSession'
import { userDate } from '../utils/userDay'
import StatePanel from '../components/ui/StatePanel.vue'
import EventCard from '../features/events/EventCard.vue'

const route = useRoute(), router = useRouter(), session = useSession()
const currentDate = session.user.value?.current_local_date || userDate(new Date(), session.user.value?.timezone || 'Europe/Moscow', session.user.value?.settings?.day_start_time)
const fallbackMonth = currentDate.slice(0, 7)
const allLayers: Array<[CalendarLayer, string]> = [
  ['pain', 'Боль'],
  ['medication', 'Лекарства'],
  ['activity', 'Активность'],
  ['sleep', 'Сон'],
  ['wellbeing', 'Самочувствие'],
  ['context', 'Контекст'],
  ['weather', 'Погода'],
]
const defaultLayers: CalendarLayer[] = ['pain', 'medication', 'activity', 'sleep', 'wellbeing', 'context', 'weather']
const month = computed(() => String(route.params.month || fallbackMonth))
const layers = computed<CalendarLayer[]>(() => {
  const raw = String(route.query.layers || '')
  if (!raw) {
    const legacy = String(route.query.mode || '')
    if (legacy && legacy !== 'overview' && defaultLayers.includes(legacy as CalendarLayer)) return [legacy as CalendarLayer]
    return defaultLayers
  }
  return raw.split(',').map((item) => item.trim()).filter((item): item is CalendarLayer => defaultLayers.includes(item as CalendarLayer))
})
const selected = computed(() => String(route.query.day || ''))
const { data, loading, error, load } = useAsyncState(() => calendarApi.month(month.value))
const preview = useAsyncState(() => journalApi.dayPreview(selected.value))

const cells = computed(() => {
  const [year, m] = month.value.split('-').map(Number)
  const first = new Date(year, m - 1, 1)
  const blanks = (first.getDay() + 6) % 7
  const days = new Date(year, m, 0).getDate()
  const map = new Map(data.value?.days.map((day) => [day.date, day]))
  return [...Array(blanks).fill(null), ...Array.from({ length: days }, (_, i) => map.get(`${month.value}-${String(i + 1).padStart(2, '0')}`) || { date: `${month.value}-${String(i + 1).padStart(2, '0')}`, has_data: false })]
})

type Signal = { key: CalendarLayer; icon: typeof HeartPulse; label: string; tone: string }

function active(layer: CalendarLayer) {
  return layers.value.includes(layer)
}

function signals(day: CalendarDay): Signal[] {
  const items: Signal[] = []
  if (active('pain') && day.pain && (day.pain.max_intensity != null || day.pain.open)) {
    items.push({ key: 'pain', icon: HeartPulse, label: day.pain.max_intensity == null ? '?' : String(day.pain.max_intensity), tone: 'pain' })
  }
  if (active('medication') && day.medication?.intakes) {
    items.push({ key: 'medication', icon: Pill, label: String(day.medication.intakes), tone: 'medication' })
  }
  if (active('activity') && day.activity?.minutes != null) {
    items.push({ key: 'activity', icon: Activity, label: `${day.activity.minutes}м`, tone: 'activity' })
  }
  if (active('sleep') && day.sleep?.minutes != null) {
    items.push({ key: 'sleep', icon: Bed, label: `${Math.round(Number(day.sleep.minutes) / 60)}ч`, tone: 'sleep' })
  }
  if (active('wellbeing') && (day.wellbeing?.score != null || day.wellbeing?.motivation != null)) {
    const score = day.wellbeing?.motivation ?? day.wellbeing?.score
    items.push({ key: 'wellbeing', icon: Smile, label: String(score), tone: 'wellbeing' })
  }
  if (active('weather') && day.weather?.temp_mean_c != null) {
    items.push({ key: 'weather', icon: CloudSun, label: `${Math.round(Number(day.weather.temp_mean_c))}°`, tone: 'weather' })
  }
  return items
}

function stripeLayers(day: CalendarDay) {
  const tones: string[] = []
  if (active('pain') && day.pain) tones.push('pain')
  if (active('medication') && day.medication?.intakes) tones.push('medication')
  if (active('activity') && day.activity?.minutes != null) tones.push('activity')
  if (active('sleep') && day.sleep?.minutes != null) tones.push('sleep')
  if (active('wellbeing') && (day.wellbeing?.score != null || day.wellbeing?.motivation != null)) tones.push('wellbeing')
  if (active('weather') && day.weather) tones.push('weather')
  if (active('context') && day.context) tones.push('context')
  return tones.slice(0, 4)
}

function metric(day: CalendarDay) {
  if (!day.has_data && !day.has_pending) return 'Нет записей'
  const all = signals(day)
  const context = active('context') && day.context
    ? `${day.context.place_label || contextTypeLabel(day.context.period_type)}`
    : ''
  return [context, ...all.map((item) => item.label)].filter(Boolean).join(' · ') || 'Есть запись'
}

function visibleSignals(day: CalendarDay) {
  const all = signals(day)
  return { shown: all.slice(0, 3), overflow: Math.max(0, all.length - 3) }
}

function contextTypeLabel(periodType?: string) {
  switch (periodType) {
    case 'trip': return 'Поездка'
    case 'vacation': return 'Отпуск'
    case 'temporary_stay': return 'Временное пребывание'
    case 'relocation': return 'Переезд'
    default: return periodType || 'Контекст'
  }
}

function toggleLayer(layer: CalendarLayer) {
  const next = new Set(layers.value)
  if (next.has(layer)) next.delete(layer)
  else next.add(layer)
  const value = [...next]
  router.replace({
    query: {
      ...route.query,
      mode: undefined,
      layers: value.length ? value.join(',') : undefined,
    },
  })
}

function move(delta: number) {
  const [y, m] = month.value.split('-').map(Number)
  const index = y * 12 + m - 1 + delta
  const target = `${Math.floor(index / 12)}-${String(index % 12 + 1).padStart(2, '0')}`
  router.push({ path: `/calendar/${target}`, query: { ...route.query } })
}

function goToday() {
  router.push({ path: `/calendar/${fallbackMonth}`, query: { layers: layers.value.join(','), day: currentDate } })
}

const addEntryTo = computed(() => `/entries/new?date=${selected.value || currentDate}`)

watch(month, () => load())
watch(selected, (date) => { if (date) preview.load() })
onMounted(() => load())
onMounted(() => { if (selected.value) preview.load() })
</script>
<template>
  <div class="page calendar-page">
    <header class="calendar-header">
      <div>
        <p class="eyebrow">История</p>
        <h1>{{ new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric' }).format(new Date(`${month}-02`)) }}</h1>
      </div>
      <div class="cluster header-actions">
        <button class="button button--secondary icon-nav" aria-label="Предыдущий месяц" @click="move(-1)"><ChevronLeft :size="18" /></button>
        <button class="button button--secondary" @click="goToday">Сегодня</button>
        <button class="button button--secondary icon-nav" aria-label="Следующий месяц" @click="move(1)"><ChevronRight :size="18" /></button>
        <RouterLink class="button icon-nav add-entry" :to="addEntryTo" aria-label="Добавить запись" title="Добавить запись">
          <Plus :size="20" />
        </RouterLink>
      </div>
    </header>
    <div class="layer-filters" aria-label="Слои календаря">
      <button
        v-for="[key, label] in allLayers"
        :key="key"
        type="button"
        class="layer-chip"
        :class="{ active: active(key), [`tone-${key}`]: true }"
        :aria-pressed="active(key)"
        @click="toggleLayer(key)"
      >
        {{ label }}
      </button>
    </div>
    <p class="legend muted">Иконка + короткое число; цветные полоски = типы записей. Погода: Open-Meteo.</p>
    <StatePanel v-if="loading" kind="loading" />
    <StatePanel v-else-if="error" kind="error" :message="error" @retry="load()" />
    <div v-else class="month-grid">
      <b v-for="day in ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']" :key="day">{{ day }}</b>
      <template v-for="(day, i) in cells" :key="day?.date || i">
        <span v-if="!day" />
        <button
          v-else
          :class="['day-cell', { selected: selected === day.date, empty: !day.has_data }]"
          :aria-label="`${day.date}: ${metric(day)}`"
          @click="$router.replace({ query: { ...$route.query, day: day.date } })"
        >
          <span v-if="stripeLayers(day).length" class="stripes" aria-hidden="true">
            <i v-for="tone in stripeLayers(day)" :key="tone" :class="`stripe tone-${tone}`" />
          </span>
          <strong>{{ Number(day.date.slice(-2)) }}</strong>
          <span
            v-if="active('context') && day.context"
            class="context-ribbon"
            :class="day.context.segment"
            :title="day.context.place_label || contextTypeLabel(day.context.period_type)"
          >
            <MapPin :size="12" aria-hidden="true" />
            <span class="signal-text">{{ day.context.place_label || contextTypeLabel(day.context.period_type) }}</span>
          </span>
          <span v-if="day.has_data" class="signals">
            <span v-for="signal in visibleSignals(day).shown" :key="signal.key" class="signal" :class="`tone-${signal.tone}`" :title="signal.label">
              <component :is="signal.icon" :size="14" aria-hidden="true" />
              <span class="signal-text">{{ signal.label }}</span>
            </span>
            <span v-if="visibleSignals(day).overflow" class="overflow">+{{ visibleSignals(day).overflow }}</span>
          </span>
          <small v-else>Нет записей</small>
          <span v-if="day.pending_count" class="pending-dot" :aria-label="`${day.pending_count} ждут проверки`" />
        </button>
      </template>
    </div>
    <aside v-if="selected" class="day-pane card">
      <h2>{{ new Date(`${selected}T12:00:00`).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' }) }}</h2>
      <p v-if="data?.days.find((d) => d.date === selected)?.context" class="muted">
        Контекст: {{ data.days.find((d) => d.date === selected)?.context?.place_label || contextTypeLabel(data.days.find((d) => d.date === selected)?.context?.period_type) }}
      </p>
      <p v-if="data?.days.find((d) => d.date === selected)?.weather" class="muted">
        Погода: {{ data.days.find((d) => d.date === selected)?.weather?.temp_mean_c != null ? `${Math.round(Number(data.days.find((d) => d.date === selected)?.weather?.temp_mean_c))}°` : 'нет данных' }}
      </p>
      <StatePanel v-if="preview.loading.value" kind="loading" />
      <StatePanel v-else-if="preview.error.value" kind="error" :message="preview.error.value" @retry="preview.load()" />
      <StatePanel v-else-if="!preview.data.value?.events.length" kind="empty" title="За этот день нет записей" />
      <div v-else class="preview-events">
        <EventCard v-for="event in preview.data.value?.events" :key="event.id" :event="event" />
      </div>
      <div class="cluster preview-actions">
        <RouterLink :to="`/day/${selected}`">Открыть полную хронологию</RouterLink>
      </div>
    </aside>
  </div>
</template>
<style scoped>
.calendar-page { display: grid; grid-template-columns: minmax(0, 1fr) 350px; gap: var(--s4); min-width: 0; }
.calendar-header, .layer-filters, .legend, .month-grid { grid-column: 1; min-width: 0; }
.calendar-header { display: flex; justify-content: space-between; align-items: flex-start; gap: var(--s3); }
.header-actions { flex-wrap: wrap; justify-content: flex-end; }
.icon-nav { display: inline-flex; align-items: center; justify-content: center; padding-inline: var(--s3); min-width: 44px; min-height: 44px; }
.add-entry { text-decoration: none; }
.layer-filters { display: flex; flex-wrap: wrap; gap: var(--s2); }
.layer-chip {
  border: 1px solid var(--border); background: var(--surface); color: var(--muted);
  border-radius: 999px; padding: 6px 10px; font-size: .8rem;
}
.layer-chip.active { color: var(--text); border-color: currentColor; font-weight: 600; }
.layer-chip.tone-pain.active { color: var(--pain); }
.layer-chip.tone-medication.active { color: var(--medication); }
.layer-chip.tone-activity.active { color: var(--activity); }
.layer-chip.tone-sleep.active { color: var(--sleep); }
.layer-chip.tone-wellbeing.active { color: var(--wellbeing); }
.layer-chip.tone-context.active { color: var(--context); }
.layer-chip.tone-weather.active { color: var(--weather); }
.legend { font-size: .75rem; margin: 0; }
.month-grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: var(--s1); }
.month-grid > b { text-align: center; color: var(--muted); font-size: .75rem; }
.day-cell {
  position: relative; min-width: 0; min-height: 92px; overflow: hidden; border: 1px solid var(--border);
  border-radius: 8px; background: var(--surface); padding: var(--s2); text-align: left;
  display: grid; align-content: start; gap: 4px;
}
.day-cell.selected { outline: 3px solid var(--primary); }
.stripes { position: absolute; left: 0; top: 0; bottom: 0; display: grid; width: 4px; }
.stripe { display: block; }
.stripe.tone-pain { background: var(--pain); }
.stripe.tone-medication { background: var(--medication); }
.stripe.tone-activity { background: var(--activity); }
.stripe.tone-sleep { background: var(--sleep); }
.stripe.tone-wellbeing { background: var(--wellbeing); }
.stripe.tone-context { background: var(--context); }
.stripe.tone-weather { background: var(--weather); }
.context-ribbon {
  display: flex; align-items: center; gap: 2px; min-width: 0; max-width: 100%;
  color: var(--context); font-size: .65rem;
  background: color-mix(in srgb, var(--context) 12%, var(--surface)); border-radius: 999px; padding: 1px 6px;
}
.context-ribbon :deep(svg) { flex-shrink: 0; }
.context-ribbon .signal-text { min-width: 0; flex: 1 1 auto; }
.signals { display: grid; gap: 2px; min-width: 0; }
.signal { display: flex; align-items: center; gap: 4px; min-width: 0; color: var(--muted); font-size: .7rem; font-variant-numeric: tabular-nums; }
.signal.tone-pain { color: var(--pain); }
.signal.tone-medication { color: var(--medication); }
.signal.tone-activity { color: var(--activity); }
.signal.tone-sleep { color: var(--sleep); }
.signal.tone-wellbeing { color: var(--wellbeing); }
.signal.tone-weather { color: var(--weather); }
.signal-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.overflow { color: var(--muted); font-size: .7rem; font-weight: 700; }
.day-cell small { color: var(--muted); overflow-wrap: anywhere; font-size: .65rem; }
.pending-dot { position: absolute; top: 8px; right: 8px; width: 8px; height: 8px; border-radius: 50%; background: var(--danger); }
.day-pane { grid-column: 2; grid-row: 1 / 6; }
.preview-events { display: grid; gap: var(--s3); }
.preview-actions { margin-top: var(--s4); }
@media (max-width: 900px) {
  .calendar-page { display: block; }
  .calendar-page > * { margin-bottom: var(--s4); }
  .day-pane { display: block; }
}
@media (max-width: 520px) {
  .calendar-header { align-items: center; }
  .day-cell { min-height: 64px; padding: 4px 4px 4px 8px; }
  .signal-text { display: none; }
}
</style>
