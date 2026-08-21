import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import WeatherComparison from './WeatherComparison.vue'
import type { WeatherAnalytics } from '../../api/types'

describe('WeatherComparison', () => {
  it('показывает сырые совпадения и прогресс до порога', () => {
    const weather: WeatherAnalytics = {
      status: 'descriptive',
      coverage: { weather_days: 31, pressure_days: 30, headache_start_days: 2, headache_starts: 2 },
      series: [{ date: '2026-08-01', pressure_delta_24h_hpa: -7, is_complete: true, pain_starts: 1 }],
      pain_windows: [],
      factors: [{
        key: 'pressure_drop', label: 'Падение давления', status: 'insufficient_data', exposed_days: 3, unexposed_days: 27,
        exposed_start_days: 1, unexposed_start_days: 1, exposed_rate: 1 / 3, unexposed_rate: 1 / 27,
        description: 'Пока недостаточно данных', requirements: {},
      }],
      formula_version: 'weather-v1', limitation: 'Не доказывает причинную связь',
    }
    const wrapper = mount(WeatherComparison, { props: { weather } })
    expect(wrapper.text()).toContain('Погода и боль')
    expect(wrapper.text()).toContain('30 из 56 дней')
    expect(wrapper.text()).toContain('1 из 3')
    expect(wrapper.find('.pain-point').exists()).toBe(true)
  })
})
