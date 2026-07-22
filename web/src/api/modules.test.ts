import { beforeEach, describe, expect, it, vi } from 'vitest'
import { authApi } from './auth'
import { journalApi } from './journal'
import { settingsApi } from './settings'

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

  it('создаёт web-запись с ключом идемпотентности', async () => {
    fetcher.mockResolvedValue(new Response(JSON.stringify({ entry_id: 'entry', status: 'queued' }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    await journalApi.createEntry('болит голова', 'request-123')
    expect(fetcher).toHaveBeenCalledWith('/api/v1/entries', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ text: 'болит голова' }),
      headers: expect.objectContaining({ 'Idempotency-Key': 'request-123' }),
    }))
  })
})
