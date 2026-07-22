import { API_BASE, json, request } from './client'
import type { Session, UserSettings } from './types'
type SettingsUpdate = Pick<Session, 'timezone' | 'locale'> & { settings?: UserSettings }
export const settingsApi = {
  update: (body: Partial<SettingsUpdate>) => request<Session>(`${API_BASE}/me`, json('PATCH', body)),
  deleteAccount: () => request<void>(`${API_BASE}/me/deletion-request`, json('POST', { confirm: 'DELETE_MY_DATA' })),
  exportUrl: (format: 'json' | 'csv') => `${API_BASE}/exports?format=${format}`,
}
