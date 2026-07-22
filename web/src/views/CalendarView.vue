<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { calendarApi, type CalendarMode } from '../api/calendar'
import type { CalendarDay } from '../api/types'
import { useAsyncState } from '../composables/useAsyncState'
import StatePanel from '../components/ui/StatePanel.vue'
const route = useRoute(), router = useRouter(), now = new Date(), fallbackMonth = now.toISOString().slice(0,7)
const modes: Array<[CalendarMode,string]> = [['overview','Обзор'],['pain','Боль'],['medication','Лекарства'],['activity','Активность'],['sleep','Сон'],['wellbeing','Самочувствие']]
const month = computed(() => String(route.params.month || fallbackMonth)), mode = computed(() => (route.query.mode || 'overview') as CalendarMode), selected = computed(() => String(route.query.day || ''))
const { data, loading, error, load } = useAsyncState(() => calendarApi.month(month.value, mode.value))
const cells = computed(() => {
  const [year, m] = month.value.split('-').map(Number), first = new Date(year, m - 1, 1), blanks = (first.getDay() + 6) % 7, days = new Date(year, m, 0).getDate()
  const map = new Map(data.value?.days.map((day) => [day.date, day]))
  return [...Array(blanks).fill(null), ...Array.from({length:days},(_,i) => map.get(`${month.value}-${String(i+1).padStart(2,'0')}`) || { date:`${month.value}-${String(i+1).padStart(2,'0')}`, has_data:false })]
})
function metric(day: CalendarDay) {
  if (!day.has_data) return 'Нет записей'
  if (mode.value === 'pain') return `${day.pain?.episodes ?? 0} эп. · ${day.pain?.max_intensity ?? '?'}/10`
  if (mode.value === 'medication') return `${day.medication?.intakes ?? 0} приём.`
  if (mode.value === 'activity') return day.activity?.minutes == null ? 'Время не указано' : `${day.activity.minutes} мин`
  if (mode.value === 'sleep') return day.sleep?.minutes == null ? 'Сон: нет данных' : `${Math.round(day.sleep.minutes/60)} ч`
  if (mode.value === 'wellbeing') return day.wellbeing?.score == null ? 'Оценка не указана' : `${day.wellbeing.score}/10`
  return [day.pain?.episodes ? `Боль ${day.pain.episodes}`:'',day.medication?.intakes ? `Лек. ${day.medication.intakes}`:''].filter(Boolean).join(' · ') || 'Есть запись'
}
function move(delta:number){const [y,m]=month.value.split('-').map(Number), d=new Date(y,m-1+delta,1);router.push({path:`/calendar/${d.toISOString().slice(0,7)}`,query:{...route.query}})}
watch([month,mode],()=>load());onMounted(()=>load())
</script>
<template><div class="page calendar-page"><header class="calendar-header"><div><p class="eyebrow">История</p><h1>{{ new Intl.DateTimeFormat('ru-RU',{month:'long',year:'numeric'}).format(new Date(`${month}-02`)) }}</h1></div><div class="cluster"><button class="button button--secondary" aria-label="Предыдущий месяц" @click="move(-1)">←</button><button class="button button--secondary" @click="$router.push('/calendar')">Сегодня</button><button class="button button--secondary" aria-label="Следующий месяц" @click="move(1)">→</button></div></header><div class="segmented" aria-label="Режим календаря"><button v-for="[key,label] in modes" :key="key" :class="{active:mode===key}" @click="$router.replace({query:{...$route.query,mode:key}})">{{ label }}</button></div><StatePanel v-if="loading" kind="loading" /><StatePanel v-else-if="error" kind="error" :message="error" @retry="load()" /><div v-else class="month-grid"><b v-for="day in ['Пн','Вт','Ср','Чт','Пт','Сб','Вс']" :key="day">{{ day }}</b><template v-for="(day,i) in cells" :key="day?.date||i"><span v-if="!day" /><button v-else :class="{selected:selected===day.date}" :aria-label="`${day.date}: ${metric(day)}`" @click="$router.replace({query:{...$route.query,day:day.date}})"><strong>{{ Number(day.date.slice(-2)) }}</strong><small>{{ metric(day) }}</small></button></template></div><aside v-if="selected" class="day-pane card"><h2>{{ new Date(`${selected}T12:00:00`).toLocaleDateString('ru-RU',{day:'numeric',month:'long'}) }}</h2><p>Выбранный день сохранён в адресе.</p><RouterLink :to="`/day/${selected}`">Открыть хронологию</RouterLink></aside></div></template>
<style scoped>.calendar-page{display:grid;grid-template-columns:minmax(0,1fr) 350px;gap:var(--s4);min-width:0}.calendar-header,.segmented,.month-grid{grid-column:1;min-width:0}.calendar-header{display:flex;justify-content:space-between}.month-grid{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:var(--s1)}.month-grid>b{text-align:center;color:var(--muted);font-size:.75rem}.month-grid button{min-width:0;min-height:92px;overflow:hidden;border:1px solid var(--border);border-radius:8px;background:var(--surface);padding:var(--s2);text-align:left;display:grid;align-content:start;gap:var(--s2)}.month-grid button.selected{outline:3px solid var(--primary)}.month-grid small{color:var(--muted);overflow-wrap:anywhere}.day-pane{grid-column:2;grid-row:1/5}@media(max-width:900px){.calendar-page{display:block}.calendar-page>*{margin-bottom:var(--s4)}.day-pane{display:none}}@media(max-width:520px){.calendar-header{display:grid}.calendar-header .cluster{margin-bottom:var(--s2)}.month-grid button{min-height:64px;padding:4px}.month-grid small{font-size:.6rem}}</style>
