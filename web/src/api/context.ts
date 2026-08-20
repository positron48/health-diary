import { API_BASE, json, request } from './client'
import type { ContextPeriod, ContextPeriodsResponse, ContextPeriodType, PlaceCandidate } from './types'

export type CreateContextPeriod = PlaceCandidate & {
  period_type: ContextPeriodType
  place_label: string
  started_on: string
  ended_on?: string
}

export const contextApi = {
  list: () => request<ContextPeriodsResponse>(`${API_BASE}/context-periods`),
  searchPlaces: (query: string) => request<{ places: PlaceCandidate[] }>(`${API_BASE}/places/search?q=${encodeURIComponent(query)}`),
  createPeriod: (body: CreateContextPeriod) => request<ContextPeriod>(`${API_BASE}/context-periods`, json('POST', body)),
  returnHome: (startedOn: string) => request<{ active: ContextPeriodsResponse['active'] }>(`${API_BASE}/context-periods/return-home`, json('POST', { started_on: startedOn })),
  cancelPeriod: (period: ContextPeriod) => request<void>(`${API_BASE}/context-periods/${period.id}`, json('PATCH', { revision: period.revision, status: 'cancelled' })),
}
