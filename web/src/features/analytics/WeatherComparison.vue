<script setup lang="ts">
import { computed } from 'vue'
import type { WeatherAnalytics, WeatherFactor } from '../../api/types'

const props = defineProps<{ weather: WeatherAnalytics }>()
const chartDays = computed(() => props.weather.series.filter((day) => day.is_complete && day.pressure_delta_24h_hpa != null))
const chart = computed(() => {
  const days = chartDays.value
  if (!days.length) return { path: '', zero: 80, points: [] as Array<{ x: number; y: number; pain: boolean; date: string; value: number }> }
  const values = days.map((day) => Number(day.pressure_delta_24h_hpa))
  const bound = Math.max(5, ...values.map(Math.abs))
  const points = days.map((day, index) => {
    const x = days.length === 1 ? 300 : 18 + index * (564 / (days.length - 1))
    const value = Number(day.pressure_delta_24h_hpa)
    return { x, y: 80 - value / bound * 62, pain: day.pain_starts > 0, date: day.date, value }
  })
  return { path: points.map((point, index) => `${index ? 'L' : 'M'}${point.x.toFixed(1)},${point.y.toFixed(1)}`).join(' '), zero: 80, points }
})
const progress = computed(() => Math.min(100, Math.round((props.weather.coverage.pressure_days / 56) * 100)))

function percent(value?: number | null) {
  return value == null ? '—' : `${Math.round(value * 100)}%`
}

function factorTitle(factor: WeatherFactor) {
  if (factor.status === 'possible_association') return 'Возможная связь'
  if (factor.status === 'no_increased_rate') return 'Учащения не видно'
  if (factor.status === 'disabled') return 'Расчёт отключён'
  return 'Пока мало данных'
}

function signed(value?: number | null) {
  if (value == null) return 'нет данных'
  return `${value > 0 ? '+' : ''}${Number(value).toFixed(1)} гПа`
}
</script>

<template>
  <section class="card weather-card">
    <header>
      <div>
        <p class="eyebrow">Описательное сравнение</p>
        <h2>Погода и боль</h2>
      </div>
      <span class="coverage-badge">{{ weather.coverage.pressure_days }} дней с давлением</span>
    </header>
    <p class="muted">
      Начал боли: {{ weather.coverage.headache_starts }}. Линия показывает изменение среднего давления за сутки; точки отмечают дни с подтверждённым началом боли.
    </p>
    <div v-if="chart.path" class="weather-chart" role="img" :aria-label="`График перепадов давления за ${weather.coverage.pressure_days} дней; начал боли ${weather.coverage.headache_starts}`">
      <svg viewBox="0 0 600 160" preserveAspectRatio="none" aria-hidden="true">
        <line x1="12" :y1="chart.zero" x2="588" :y2="chart.zero" class="zero-line" />
        <path :d="chart.path" class="pressure-line" />
        <circle v-for="point in chart.points.filter((item) => item.pain)" :key="point.date" :cx="point.x" :cy="point.y" r="5" class="pain-point">
          <title>{{ point.date }}: {{ signed(point.value) }}, начало боли</title>
        </circle>
      </svg>
      <div class="chart-legend"><span><i class="line-key" /> Перепад давления</span><span><i class="dot-key" /> Начало боли</span></div>
    </div>
    <p v-else class="muted">Почасовой backfill поставлен в очередь. График появится после обработки погоды.</p>

    <div v-if="weather.coverage.pressure_days < 56 || weather.coverage.headache_start_days < 8" class="gate">
      <strong>Для вывода о возможной связи пока мало данных</strong>
      <div class="progress" role="progressbar" aria-label="Покрытие погодой" :aria-valuenow="progress" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${progress}%` }" /></div>
      <p class="muted">{{ weather.coverage.pressure_days }} из 56 дней · {{ weather.coverage.headache_start_days }} из 8 дней с началом боли. Сырые совпадения уже можно смотреть.</p>
    </div>

    <div class="factor-grid">
      <article v-for="factor in weather.factors" :key="factor.key" class="factor">
        <span class="factor-status">{{ factorTitle(factor) }}</span>
        <h3>{{ factor.label }}</h3>
        <p><b>{{ factor.exposed_start_days }} из {{ factor.exposed_days }}</b> дней с фактором · <b>{{ factor.unexposed_start_days }} из {{ factor.unexposed_days }}</b> остальных</p>
        <p class="muted">Частота записанных начал: {{ percent(factor.exposed_rate) }} против {{ percent(factor.unexposed_rate) }}.</p>
        <p class="muted">{{ factor.description }}</p>
      </article>
    </div>

    <details v-if="weather.pain_windows.length" class="pain-windows">
      <summary>Давление непосредственно перед началами боли</summary>
      <ul>
        <li v-for="item in weather.pain_windows.slice(-8).reverse()" :key="item.occurred_at">
          {{ new Date(item.occurred_at).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }) }}:
          за 6 ч {{ signed(item.pressure_delta_6h_hpa) }}, за 24 ч {{ signed(item.pressure_delta_24h_hpa) }}<span v-if="item.intensity != null">, боль {{ item.intensity }}/10</span>
        </li>
      </ul>
    </details>
    <p class="limitation">{{ weather.limitation }}. Версия формулы: {{ weather.formula_version }}.</p>
  </section>
</template>

<style scoped>
.weather-card { display: grid; gap: var(--s4); }
.weather-card header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s3); }
.coverage-badge, .factor-status { display: inline-flex; border-radius: 999px; padding: 4px 9px; background: #edf1f3; color: var(--muted); font-size: .75rem; }
.weather-chart { border: 1px solid var(--border); border-radius: var(--radius-control); padding: var(--s3); background: #fafcfc; }
.weather-chart svg { width: 100%; height: 180px; overflow: visible; }
.zero-line { stroke: var(--border); stroke-width: 1; stroke-dasharray: 5 5; }
.pressure-line { fill: none; stroke: var(--weather); stroke-width: 3; vector-effect: non-scaling-stroke; }
.pain-point { fill: var(--pain); stroke: white; stroke-width: 2; vector-effect: non-scaling-stroke; }
.chart-legend { display: flex; gap: var(--s4); flex-wrap: wrap; color: var(--muted); font-size: .8rem; }
.chart-legend span { display: inline-flex; align-items: center; gap: var(--s2); }
.line-key { width: 22px; border-top: 3px solid var(--weather); }
.dot-key { width: 10px; height: 10px; border-radius: 50%; background: var(--pain); }
.gate { padding: var(--s4); background: #f5f7f4; border-radius: var(--radius-control); }
.progress { height: 7px; margin: var(--s3) 0; overflow: hidden; border-radius: 99px; background: var(--border); }
.progress i { display: block; height: 100%; background: var(--weather); }
.factor-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: var(--s3); }
.factor { padding: var(--s4); border: 1px solid var(--border); border-radius: var(--radius-control); }
.factor h3 { margin: var(--s2) 0; }
.factor p { margin-bottom: var(--s2); }
.pain-windows ul { padding-left: var(--s5); }
.limitation { margin: 0; color: var(--muted); font-size: .8rem; }
@media(max-width:700px) { .weather-card header { display: block; } .coverage-badge { margin-bottom: var(--s3); } .weather-chart svg { height: 140px; } }
</style>
