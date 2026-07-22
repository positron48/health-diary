import { Activity, Bed, CircleGauge, Coffee, FileText, HeartPulse, Pill, Smile } from '@lucide/vue'
import type { Component } from 'vue'
import type { HealthEvent } from '../../api/types'

type Descriptor = {
  label: string
  icon: Component
  tone: string
  fields: Array<{ key: string; label: string; format?: (value: unknown, data?: Record<string, unknown>) => string }>
}

const value = (v: unknown) => (v === null || v === undefined || v === '' ? 'Не указано' : String(v))
const score = (v: unknown) => `${value(v)}/10`
const list = (v: unknown) => (Array.isArray(v) ? v.map(String).filter(Boolean).join(', ') : value(v))

const locationLabels: Record<string, string> = {
  top_of_head: 'верхняя часть головы',
  upper_head: 'верхняя часть головы',
  occiput_neck: 'затылок/шея',
  occiput: 'затылок',
  neck: 'шея',
  temple: 'висок',
  temporal: 'висок',
  forehead: 'лоб',
  frontal: 'лоб',
  right_side: 'правая сторона',
  left_side: 'левая сторона',
  head: 'голова',
}

const formatLocations = (v: unknown) => {
  if (!Array.isArray(v) || !v.length) return value(v)
  return v.map((item) => locationLabels[String(item)] || String(item)).join(', ')
}

const phaseLabel = (phase: unknown) => {
  switch (phase) {
    case 'start': return 'началась'
    case 'update': return 'наблюдение'
    case 'end': return 'прошла'
    default: return ''
  }
}

const lateralityFormat = (v: unknown) =>
  ({ left: 'слева', right: 'справа', bilateral: 'с обеих сторон', center: 'по центру', unknown: 'Не указано' }[String(v)] || value(v))

export const eventRegistry: Record<string, Descriptor> = {
  pain: {
    label: 'Головная боль',
    icon: HeartPulse,
    tone: 'pain',
    fields: [
      { key: 'intensity', label: 'Интенсивность', format: score },
      { key: 'locations', label: 'Область', format: formatLocations },
      { key: 'location', label: 'Область' },
      { key: 'laterality', label: 'Сторона', format: lateralityFormat },
      { key: 'qualities', label: 'Характер', format: list },
      { key: 'associated_symptoms', label: 'Симптомы', format: list },
      { key: 'symptoms', label: 'Симптомы', format: list },
      { key: 'functional_impact', label: 'Влияние' },
    ],
  },
  pain_observation: {
    label: 'Головная боль',
    icon: HeartPulse,
    tone: 'pain',
    fields: [
      { key: 'intensity', label: 'Интенсивность', format: score },
      { key: 'locations', label: 'Область', format: formatLocations },
      { key: 'laterality', label: 'Сторона', format: lateralityFormat },
      { key: 'qualities', label: 'Характер', format: list },
      { key: 'associated_symptoms', label: 'Симптомы', format: list },
      { key: 'functional_impact', label: 'Влияние' },
    ],
  },
  medication_intake: {
    label: 'Приём лекарства',
    icon: Pill,
    tone: 'medication',
    fields: [
      { key: 'name_raw', label: 'Название' },
      { key: 'normalized_name', label: 'Нормализовано' },
      { key: 'name', label: 'Название' },
      {
        key: 'dose_value',
        label: 'Доза',
        format: (v, data) => (v == null || v === '' ? 'Не указано' : `${value(v)}${data?.dose_unit ? ` ${data.dose_unit}` : ''}`),
      },
      { key: 'effect_rating', label: 'Эффект' },
    ],
  },
  sleep: {
    label: 'Сон',
    icon: Bed,
    tone: 'sleep',
    fields: [
      { key: 'duration_minutes', label: 'Продолжительность', format: (v) => `${value(v)} мин` },
      { key: 'quality_score', label: 'Качество', format: score },
      { key: 'quality', label: 'Качество', format: score },
    ],
  },
  activity: {
    label: 'Активность',
    icon: Activity,
    tone: 'activity',
    fields: [
      { key: 'activity_type', label: 'Вид' },
      { key: 'duration_minutes', label: 'Продолжительность', format: (v) => `${value(v)} мин` },
    ],
  },
  wellbeing: {
    label: 'Самочувствие',
    icon: Smile,
    tone: 'wellbeing',
    fields: [
      { key: 'wellbeing_score', label: 'Оценка', format: score },
      { key: 'score', label: 'Оценка', format: score },
      { key: 'note', label: 'Комментарий' },
    ],
  },
  food_drink: {
    label: 'Еда и напитки',
    icon: Coffee,
    tone: 'food',
    fields: [{ key: 'description', label: 'Запись' }, { key: 'category', label: 'Категория' }],
  },
  measurement: {
    label: 'Измерение',
    icon: CircleGauge,
    tone: 'measurement',
    fields: [
      { key: 'measurement_type', label: 'Показатель' },
      { key: 'name', label: 'Показатель' },
      { key: 'value', label: 'Значение' },
      { key: 'unit', label: 'Единица' },
    ],
  },
  note: {
    label: 'Заметка',
    icon: FileText,
    tone: 'note',
    fields: [{ key: 'text', label: 'Заметка' }],
  },
}

export const descriptorFor = (kind: string, data: Record<string, unknown> = {}) => {
  const base = eventRegistry[kind] || { label: 'Событие', icon: FileText, tone: 'note', fields: [] }
  if (kind === 'pain_observation' || kind === 'pain') {
    const phase = phaseLabel(data.phase)
    const headache = data.symptom_type === 'headache' || !data.symptom_type
    return {
      ...base,
      label: headache ? (phase ? `Головная боль · ${phase}` : 'Головная боль') : (phase ? `Боль · ${phase}` : 'Боль'),
    }
  }
  if (kind === 'medication_intake') {
    const name = data.name_raw || data.normalized_name || data.name
    if (typeof name === 'string' && name.trim()) {
      return { ...base, label: `Приём: ${name}` }
    }
  }
  return base
}

export function eventFields(event: HealthEvent) {
  const data = event.data || event.attributes || {}
  const descriptor = descriptorFor(event.kind, data)
  const seen = new Set<string>()
  return descriptor.fields
    .filter((field) => {
      if (data[field.key] === undefined || seen.has(field.label)) return false
      if (field.key === 'name' && data.name_raw !== undefined) return false
      if (field.key === 'normalized_name' && data.name_raw) return false
      if (field.key === 'dose_unit') return false
      seen.add(field.label)
      return true
    })
    .map((field) => ({ label: field.label, value: field.format?.(data[field.key], data) ?? value(data[field.key]) }))
}

export const eventTime = (event: HealthEvent) =>
  `${new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' }).format(new Date(event.occurred_at))}${
    event.time_precision === 'approximate' || event.time_precision === 'inferred_from_message'
      ? ' · примерно'
      : event.time_precision === 'unknown' || event.time_precision === 'date_only'
        ? ' · время не указано'
        : ''
  }`

export const entryIdOf = (event: HealthEvent) => event.entry_id || event.source_entry_id || ''
