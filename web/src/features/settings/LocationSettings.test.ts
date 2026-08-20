import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { contextApi } from '../../api/context'
import { ApiError } from '../../api/client'
import { useSession } from '../../composables/useSession'
import LocationSettings from './LocationSettings.vue'

const lipetsk = {
  provider_place_id: '27960', label: 'Липецк', region: 'Липецкая область', country_code: 'RU',
  timezone: 'Europe/Moscow', latitude: 52.6088, longitude: 39.5992,
}

describe('LocationSettings', () => {
  beforeEach(() => {
    useSession().authenticated({
      id: 'user', timezone: 'Europe/Moscow', current_local_date: '2026-08-20',
      settings: { day_start_time: '00:00', home_place_id: 'home' },
    })
    vi.spyOn(contextApi, 'list').mockResolvedValue({
      active: { place_id: 'home', place_label: 'Москва', source: 'home', local_date: '2026-08-20' },
      periods: [],
    })
  })

  it('создаёт выбранный город с включительной даты', async () => {
    vi.spyOn(contextApi, 'searchPlaces').mockResolvedValue({ places: [lipetsk] })
    const create = vi.spyOn(contextApi, 'createPeriod').mockResolvedValue({
      id: 'period', period_type: 'temporary_stay', place_id: 'lipetsk', place_label: 'Липецк',
      started_on: '2026-08-15', status: 'open', revision: 1,
    })
    const wrapper = mount(LocationSettings, { attachTo: document.body })
    await flushPromises()
    await wrapper.get('[data-testid="location-query"]').setValue('Липецк')
    await wrapper.get('[data-testid="location-search"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="location-hit"]').trigger('click')
    await wrapper.get('[data-testid="location-start"]').setValue('2026-08-15')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith(expect.objectContaining({
      provider_place_id: '27960', place_label: 'Липецк', started_on: '2026-08-15', period_type: 'temporary_stay',
    }))
    wrapper.unmount()
  })

  it('возвращает настроенный домашний город с выбранной даты', async () => {
    const returnHome = vi.spyOn(contextApi, 'returnHome').mockResolvedValue({
      active: { place_id: 'home', place_label: 'Москва', source: 'home', local_date: '2026-08-18' },
    })
    const wrapper = mount(LocationSettings)
    await flushPromises()
    await wrapper.get('[data-testid="location-start"]').setValue('2026-08-18')
    await wrapper.get('[data-testid="return-home"]').trigger('click')
    const confirm = document.querySelector<HTMLButtonElement>('[data-testid="confirm-return-home"]')
    expect(confirm).not.toBeNull()
    confirm!.click()
    await flushPromises()
    expect(returnHome).toHaveBeenCalledWith('2026-08-18')
    wrapper.unmount()
  })

  it('сохраняет черновик и показывает конфликт периодов', async () => {
    vi.spyOn(contextApi, 'searchPlaces').mockResolvedValue({ places: [lipetsk] })
    vi.spyOn(contextApi, 'createPeriod').mockRejectedValue(new ApiError('Конфликт', 409, 'context_period_conflict'))
    const wrapper = mount(LocationSettings)
    await flushPromises()
    await wrapper.get('[data-testid="location-query"]').setValue('Липецк')
    await wrapper.get('[data-testid="location-search"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="location-hit"]').trigger('click')
    await wrapper.get('[data-testid="location-start"]').setValue('2026-08-15')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('На эти даты уже задан другой город')
    expect((wrapper.get('[data-testid="location-query"]').element as HTMLInputElement).value).toBe('Липецк')
    expect((wrapper.get('[data-testid="location-start"]').element as HTMLInputElement).value).toBe('2026-08-15')
    wrapper.unmount()
  })
})
