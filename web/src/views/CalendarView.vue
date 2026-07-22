<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Activity, Bed, ChevronLeft, ChevronRight, HeartPulse, Pill, Smile } from '@lucide/vue'
import { calendarApi, type CalendarMode } from '../api/calendar'
import type { CalendarDay } from '../api/types'
import { useAsyncState } from '../composables/useAsyncState'
import StatePanel from '../components/ui/StatePanel.vue'

const route = useRoute(), router = useRouter(), now = new Date(), fallbackMonth = now.toISOString().slice(0, 7)
const modes: Array<[CalendarMode, string]> = [['overview', 'Обзор'], ['pain', 'Боль'], ['medication', 'Лекарства'], ['activity', 'Активность'], ['sleep', 'Сон'], ['wellbeing', 'Самочувствие']]
const month = computed(() => String(route.params.month || fallbackMonth))
const mode = computed(() => (route.query.mode || 'overview') as CalendarMode)
const selected = computed(() => String(route.query.day || ''))
const { data, loading, error, load } = useAsyncState(() => calendarApi.month(month.value, mode.value))

const cells = computed(() => {
  const [year, m] = month.value.split('-').map(Number)
  const first = new Date(year, m - 1, 1)
  const blanks = (first.getDay() + 6) % 7
  const days = new Date(year, m, 0).getDate()
  const map = new Map(data.value?.days.map((day) => [day.date, day]))
  return [...Array(blanks).fill(null), ...Array.from({ length: days }, (_, i) => map.get(`${month.value}-${String(i + 1).padStart(2, '0')}`) || { date: `${month.value}-${String(i + 1).padStart(2, '0')}`, has_data: false })]
})

type Signal = { key: string; icon: typeof HeartPulse; label: string; tone: string }

function signals(day: CalendarDay): Signal[] {
  const items: Signal[] = []
  if (day.pain?.episodes) items.push({ key: 'pain', icon: HeartPulse, label: `${day.pain.episodes} эп. · ${day.pain.max_intensity ?? '?'}/10`, tone: 'pain' })
  if (day.medication?.intakes) items.push({ key: 'medication', icon: Pill, label: `${day.medication.intakes} приём.`, tone: 'medication' })
  if (day.activity?.minutes != null) items.push({ key: 'activity', icon: Activity, label: `${day.activity.minutes} мин`, tone: 'activity' })
  if (day.sleep?.minutes != null) items.push({ key: 'sleep', icon: Bed, label: `${Math.round(day.sleep.minutes / 60)} ч`, tone: 'sleep' })
  if (day.wellbeing?.score != null) items.push({ key: 'wellbeing', icon: Smile, label: `${day.wellbeing.score}/10`, tone: 'wellbeing' })
  return items
}

function metric(day: CalendarDay) {
  if (!day.has_data) return 'Нет записей'
  if (mode.value === 'pain') return `${day.pain?.episodes ?? 0} эп. · ${day.pain?.max_intensity ?? '?'}/10`
  if (mode.value === 'medication') return `${day.medication?.intakes ?? 0} приём.`
  if (mode.value === 'activity') return day.activity?.minutes == null ? 'Время не указано' : `${day.activity.minutes} мин`
  if (mode.value === 'sleep') return day.sleep?.minutes == null ? 'Сон: нет данных' : `${Math.round(day.sleep.minutes / 60)} ч`
  if (mode.value === 'wellbeing') return day.wellbeing?.score == null ? 'Оценка не указана' : `${day.wellbeing.score}/10`
  const all = signals(day)
  if (!all.length) return 'Есть запись'
  return all.map((item) => item.label).join(' · ')
}

function visibleSignals(day: CalendarDay) {
  const all = signals(day)
  if (mode.value === 'overview') return { shown: all.slice(0, 2), overflow: Math.max(0, all.length - 2) }
  const focused = all.filter((item) => item.key === mode.value || (mode.value === 'pain' && item.key === 'pain'))
  return { shown: focused.slice(0, 2), overflow: Math.max(0, focused.length - 2) }
}

function toneClass(day: CalendarDay) {
  if (!day.has_data) return ''
  if (mode.value === 'overview') return day.pain?.episodes ? 'tone-pain' : day.medication?.intakes ? 'tone-medication' : ''
  return `tone-${mode.value}`
}

function move(delta: number) {
  const [y, m] = month.value.split('-').map(Number)
  const d = new Date(y, m - 1 + delta, 1)
  router.push({ path: `/calendar/${d.toISOString().slice(0, 7)}`, query: { ...route.query } })
}

watch([month, mode], () => load())
onMounted(() => load())
</script>
<template>
  <div class="page calendar-page">
    <header class="calendar-header">
      <div>
        <p class="eyebrow">История</p>
        <h1>{{ new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric' }).format(new Date(`${month}-02`)) }}</h1>
      </div>
      <div class="cluster">
        <button class="button button--secondary icon-nav" aria-label="Предыдущий месяц" @click="move(-1)"><ChevronLeft :size="18" /></button>
        <button class="button button--secondary" @click="$router.push('/calendar')">Сегодня</button>
        <button class="button button--secondary icon-nav" aria-label="Следующий месяц" @click="move(1)"><ChevronRight :size="18" /></button>
      </div>
    </header>
    <div class="segmented" aria-label="Режим календаря">
      <button v-for="[key, label] in modes" :key="key" :class="{ active: mode === key }" @click="$router.replace({ query: { ...$route.query, mode: key } })">{{ label }}</button>
    </div>
    <StatePanel v-if="loading" kind="loading" />
    <StatePanel v-else-if="error" kind="error" :message="error" @retry="load()" />
    <div v-else class="month-grid">
      <b v-for="day in ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']" :key="day">{{ day }}</b>
      <template v-for="(day, i) in cells" :key="day?.date || i">
        <span v-if="!day" />
        <button
          v-else
          :class="['day-cell', toneClass(day), { selected: selected === day.date, empty: !day.has_data }]"
          :aria-label="`${day.date}: ${metric(day)}`"
          @click="$router.replace({ query: { ...$route.query, day: day.date } })"
        >
          <strong>{{ Number(day.date.slice(-2)) }}</strong>
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
      <p>Выбранный день сохранён в адресе.</p>
      <RouterLink :to="`/day/${selected}`">Открыть хронологию</RouterLink>
    </aside>
  </div>
</template>
<style scoped>
.calendar-page { display: grid; grid-template-columns: minmax(0, 1fr) 350px; gap: var(--s4); min-width: 0; }
.calendar-header, .segmented, .month-grid { grid-column: 1; min-width: 0; }
.calendar-header { display: flex; justify-content: space-between; }
.icon-nav { display: inline-flex; align-items: center; justify-content: center; padding-inline: var(--s3); }
.month-grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: var(--s1); }
.month-grid > b { text-align: center; color: var(--muted); font-size: .75rem; }
.day-cell {
  position: relative; min-width: 0; min-height: 92px; overflow: hidden; border: 1px solid var(--border);
  border-radius: 8px; background: var(--surface); padding: var(--s2); text-align: left;
  display: grid; align-content: start; gap: var(--s2);
}
.day-cell.selected { outline: 3px solid var(--primary); }
.day-cell.tone-pain { background: color-mix(in srgb, var(--pain) 8%, var(--surface)); }
.day-cell.tone-medication { background: color-mix(in srgb, var(--medication) 8%, var(--surface)); }
.day-cell.tone-activity { background: color-mix(in srgb, var(--activity) 8%, var(--surface)); }
.day-cell.tone-sleep { background: color-mix(in srgb, var(--sleep) 8%, var(--surface)); }
.day-cell.tone-wellbeing { background: color-mix(in srgb, var(--wellbeing) 8%, var(--surface)); }
.signals { display: grid; gap: 4px; min-width: 0; }
.signal { display: flex; align-items: center; gap: 4px; min-width: 0; color: var(--muted); font-size: .7rem; }
.signal.tone-pain { color: var(--pain); }
.signal.tone-medication { color: var(--medication); }
.signal.tone-activity { color: var(--activity); }
.signal.tone-sleep { color: var(--sleep); }
.signal.tone-wellbeing { color: var(--wellbeing); }
.signal-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.overflow { color: var(--muted); font-size: .7rem; font-weight: 700; }
.day-cell small { color: var(--muted); overflow-wrap: anywhere; font-size: .65rem; }
.pending-dot { position: absolute; top: 8px; right: 8px; width: 8px; height: 8px; border-radius: 50%; background: var(--danger); }
.day-pane { grid-column: 2; grid-row: 1 / 5; }
@media (max-width: 900px) {
  .calendar-page { display: block; }
  .calendar-page > * { margin-bottom: var(--s4); }
  .day-pane { display: none; }
}
@media (max-width: 520px) {
  .calendar-header { display: grid; }
  .calendar-header .cluster { margin-bottom: var(--s2); }
  .day-cell { min-height: 64px; padding: 4px; }
  .signal-text { display: none; }
}
</style>
