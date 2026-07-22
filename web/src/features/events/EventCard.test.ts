import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import EventCard from './EventCard.vue'
describe('EventCard', () => {
  it('показывает русское имя, нулевое и неизвестное значения без JSON', () => {
    const wrapper = mount(EventCard, { props: { event: { id:'1',kind:'pain',occurred_at:'2026-07-22T12:00:00Z',time_precision:'approximate',revision:1,data:{intensity:0,location:null} } } })
    expect(wrapper.text()).toContain('Головная боль')
    expect(wrapper.text()).toContain('0/10')
    expect(wrapper.text()).toContain('Не указано')
    expect(wrapper.text()).toContain('примерно')
    expect(wrapper.text()).not.toContain('"intensity"')
  })
})
