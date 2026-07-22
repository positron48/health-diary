import { API_BASE, json, request } from './client'
import type { Session } from './types'
export const settingsApi = {
  update: (body: Partial<Session>) => request<Session>(`${API_BASE}/me`, json('PATCH', body)),
  deleteAccount: () => request<void>(`${API_BASE}/me/deletion-request`, json('POST', { confirm: 'DELETE_MY_DATA' })),
  exportUrl: (format: 'json' | 'csv') => `${API_BASE}/exports?format=${format}`,
}
