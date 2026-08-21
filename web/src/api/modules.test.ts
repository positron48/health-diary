import { beforeEach, describe, expect, it, vi } from 'vitest'
import { authApi } from './auth'
import { journalApi } from './journal'
import { settingsApi } from './settings'
import { contextApi } from './context'
import { analyticsApi } from './analytics'

describe('canonical API modules', () => {
  const fetcher = vi.fn()
  beforeEach(() => {
    fetcher.mockReset().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetcher)
  })

  it('использует /api/v1 для auth и export', async () => {
    await authApi.verify('challenge', '123456')
    expect(fetcher).toHaveBeenCalledWith('/api/v1/auth/challenges/challenge/verify', expect.any(Object))
    expect(settingsApi.exportUrl('csv')).toBe('/api/v1/exports?format=csv')
  })

  it('передаёт новую revision восстановления в query без body payload', async () => {
    await journalApi.restore({ id: 'event', kind: 'note', occurred_at: '2026-07-22T12:00:00Z', revision: 4 })
    expect(fetcher).toHaveBeenCalledWith('/api/v1/events/event/restore?revision=5', expect.objectContaining({ method: 'POST', body: undefined }))
  })

  it('передаёт revision при изменении эпизода', async () => {
    fetcher.mockResolvedValue(new Response(JSON.stringify({ id: 'episode', status: 'open', revision: 3 }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    await journalApi.reopenEpisode('episode', 2)
    expect(fetcher).toHaveBeenCalledWith('/api/v1/episodes/episode/reopen', expect.objectContaining({ body: JSON.stringify({ revision: 2 }) }))
  })

  it('загружает последние 10 событий выбранного пользовательского дня', async () => {
    fetcher.mockResolvedValue(new Response(JSON.stringify({ events: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    await journalApi.dayPreview('2026-07-22')
    expect(fetcher).toHaveBeenCalledWith('/api/v1/events?from=2026-07-22&to=2026-07-22&limit=10', expect.any(Object))
  })

  it('удаляет подвисшую запись входящих через DELETE /entries/{id}', async () => {
    await journalApi.deleteEntry('entry-stuck')
    expect(fetcher).toHaveBeenCalledWith('/api/v1/entries/entry-stuck', expect.objectContaining({ method: 'DELETE' }))
  })

  it('сохраняет ручное возвращение домой с локальной даты', async () => {
    fetcher.mockResolvedValue(new Response(JSON.stringify({ active: { place_id: 'home', place_label: 'Липецк', source: 'home', local_date: '2026-08-20' } }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    await contextApi.returnHome('2026-08-20')
    expect(fetcher).toHaveBeenCalledWith('/api/v1/context-periods/return-home', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ started_on: '2026-08-20' }),
    }))
  })

  it('загружает сводку и погодное сравнение за один период', async () => {
    fetcher.mockImplementation((url: string) => {
      const body = url.includes('/analytics/weather')
        ? { status: 'descriptive', coverage: { weather_days: 31, pressure_days: 30, headache_start_days: 2, headache_starts: 2 }, series: [], pain_windows: [], factors: [], formula_version: 'weather-v1', limitation: 'test' }
        : { coverage: { observation_days: 30, diary_days: 2, confirmed_events: 2, pending_events: 0, episodes: 2, closed_episodes: 2 }, metrics: {}, formula_version: 'summary-v2' }
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    })
    const result = await analyticsApi.summary(30, 'UTC', '00:00')
    expect(fetcher).toHaveBeenCalledTimes(2)
    expect(result.weather?.coverage.weather_days).toBe(31)
  })
})
