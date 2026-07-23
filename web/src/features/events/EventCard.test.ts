import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import EventCard from './EventCard.vue'
import { descriptorFor, eventFields } from './eventRegistry'

describe('EventCard', () => {
  it('показывает русское имя, нулевое и неизвестное значения без JSON', () => {
    const wrapper = mount(EventCard, {
      props: {
        event: {
          id: '1',
          kind: 'pain',
          occurred_at: '2026-07-22T12:00:00Z',
          time_precision: 'approximate',
          revision: 1,
          data: { intensity: 0, location: null },
        },
      },
    })
    expect(wrapper.text()).toContain('Головная боль')
    expect(wrapper.text()).toContain('0/10')
    expect(wrapper.text()).toContain('Не указано')
    expect(wrapper.text()).toContain('примерно')
    expect(wrapper.text()).not.toContain('"intensity"')
  })

  it('рендерит API-ключи pain_observation и medication name_raw', () => {
    const pain = {
      id: '1',
      kind: 'pain_observation',
      occurred_at: '2026-07-20T12:00:00Z',
      time_precision: 'approximate' as const,
      revision: 1,
      entry_id: 'entry-1',
      data: { symptom_type: 'headache', phase: 'start', locations: ['top_of_head'] },
    }
    const med = {
      id: '2',
      kind: 'medication_intake',
      occurred_at: '2026-07-20T16:00:00Z',
      revision: 1,
      data: { name_raw: 'цитрамон' },
    }
    expect(descriptorFor(pain.kind, pain.data).label).toContain('Головная боль')
    expect(eventFields(pain).some((field) => field.value.includes('верхняя часть головы'))).toBe(true)
    expect(descriptorFor(med.kind, med.data).label).toContain('цитрамон')
    const wrapper = mount(EventCard, { props: { event: med } })
    expect(wrapper.text()).toContain('цитрамон')
    expect(wrapper.text()).not.toContain('"name_raw"')
  })

  it('показывает поездку с городом и датой окончания из ended_at', () => {
    const trip = {
      id: '3',
      kind: 'life_context',
      occurred_at: '2026-07-19T09:00:00Z',
      ended_at: '2026-07-22T21:00:00Z',
      time_precision: 'date_only' as const,
      revision: 1,
      data: { period_type: 'trip', place_label: 'Новосибирск', phase: 'start' },
    }
    expect(descriptorFor(trip.kind, trip.data).label).toBe('Поездка · Новосибирск')
    const fields = eventFields(trip)
    expect(fields.some((field) => field.label === 'До' && field.value.includes('22'))).toBe(true)
    expect(fields.some((field) => field.label === 'Тип')).toBe(false)
    const wrapper = mount(EventCard, { props: { event: trip } })
    expect(wrapper.text()).toContain('Поездка · Новосибирск')
    expect(wrapper.text()).toContain('До')
  })
})
