import { Activity, Bed, CircleGauge, Coffee, FileText, HeartPulse, Pill, Smile } from '@lucide/vue'
import type { Component } from 'vue'
import type { HealthEvent } from '../../api/types'
type Descriptor = { label: string; icon: Component; tone: string; fields: Array<{ key: string; label: string; format?: (value: unknown) => string }> }
const value = (v: unknown) => v === null || v === undefined || v === '' ? 'Не указано' : String(v)
const score = (v: unknown) => `${value(v)}/10`
export const eventRegistry: Record<string, Descriptor> = {
  pain: { label: 'Головная боль', icon: HeartPulse, tone: 'pain', fields: [{ key: 'intensity', label: 'Интенсивность', format: score }, { key: 'location', label: 'Область' }, { key: 'symptoms', label: 'Симптомы', format: (v) => Array.isArray(v) ? v.join(', ') : value(v) }] },
  pain_observation: { label: 'Наблюдение боли', icon: HeartPulse, tone: 'pain', fields: [{ key: 'intensity', label: 'Интенсивность', format: score }] },
  medication_intake: { label: 'Приём лекарства', icon: Pill, tone: 'medication', fields: [{ key: 'name', label: 'Название' }, { key: 'dose_value', label: 'Доза', format: (v) => value(v) }, { key: 'dose_unit', label: 'Единица' }] },
  sleep: { label: 'Сон', icon: Bed, tone: 'sleep', fields: [{ key: 'duration_minutes', label: 'Продолжительность', format: (v) => `${value(v)} мин` }, { key: 'quality', label: 'Качество', format: score }] },
  activity: { label: 'Активность', icon: Activity, tone: 'activity', fields: [{ key: 'activity_type', label: 'Вид' }, { key: 'duration_minutes', label: 'Продолжительность', format: (v) => `${value(v)} мин` }] },
  wellbeing: { label: 'Самочувствие', icon: Smile, tone: 'wellbeing', fields: [{ key: 'score', label: 'Оценка', format: score }, { key: 'note', label: 'Комментарий' }] },
  food_drink: { label: 'Еда и напитки', icon: Coffee, tone: 'food', fields: [{ key: 'description', label: 'Запись' }] },
  measurement: { label: 'Измерение', icon: CircleGauge, tone: 'measurement', fields: [{ key: 'name', label: 'Показатель' }, { key: 'value', label: 'Значение' }, { key: 'unit', label: 'Единица' }] },
  note: { label: 'Заметка', icon: FileText, tone: 'note', fields: [{ key: 'text', label: 'Заметка' }] },
}
export const descriptorFor = (kind: string) => eventRegistry[kind] || { label: 'Событие', icon: FileText, tone: 'note', fields: [] }
export function eventFields(event: HealthEvent) {
  const data = event.data || event.attributes || {}, descriptor = descriptorFor(event.kind)
  return descriptor.fields.filter((field) => data[field.key] !== undefined).map((field) => ({ label: field.label, value: field.format?.(data[field.key]) ?? value(data[field.key]) }))
}
export const eventTime = (event: HealthEvent) => `${new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' }).format(new Date(event.occurred_at))}${event.time_precision === 'approximate' ? ' · примерно' : event.time_precision === 'unknown' ? ' · время не указано' : ''}`
