import { API_BASE, json, request } from './client'
import type { DayResponse, Episode, HealthEvent, PendingBatch } from './types'
export const journalApi = {
  events: (params = '') => request<{ events: HealthEvent[] }>(`${API_BASE}/events${params}`),
  event: (id: string) => request<HealthEvent>(`${API_BASE}/events/${id}`),
  day: (date: string) => request<DayResponse>(`${API_BASE}/days/${date}`),
  pending: () => request<{ batches: PendingBatch[] }>(`${API_BASE}/batches?status=pending`),
  confirm: (id: string, version: number) => request<void>(`${API_BASE}/batches/${id}/confirm`, json('POST', { version })),
  reject: (id: string, version: number) => request<void>(`${API_BASE}/batches/${id}/reject`, json('POST', { version })),
  update: (id: string, patch: Record<string, unknown>) => request<HealthEvent>(`${API_BASE}/events/${id}`, json('PATCH', patch)),
  remove: (event: HealthEvent) => request<void>(`${API_BASE}/events/${event.id}?revision=${event.revision}`, json('DELETE')),
  restore: (event: HealthEvent) => request<void>(`${API_BASE}/events/${event.id}/restore?revision=${event.revision + 1}`, json('POST')),
  episode: (id: string) => request<Episode>(`${API_BASE}/episodes/${id}`),
  closeEpisode: (id: string, revision: number, endedAt: string, precision: string) => request<Episode>(`${API_BASE}/episodes/${id}/close`, json('POST', { revision, ended_at: endedAt, precision })),
  reopenEpisode: (id: string, revision: number) => request<Episode>(`${API_BASE}/episodes/${id}/reopen`, json('POST', { revision })),
  source: (entryId: string) => request<{ id: string; source_type: string; source_sent_at: string; text: string }>(`${API_BASE}/entries/${entryId}`),
}
