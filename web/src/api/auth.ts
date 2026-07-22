import { API_BASE, json, request } from './client'
import type { Session } from './types'
export interface Challenge { challenge_id: string; telegram_deep_link?: string; telegram_url?: string; expires_at?: string }
export const authApi = {
  session: () => request<Session>(`${API_BASE}/me`),
  challenge: () => request<Challenge>(`${API_BASE}/auth/challenges`, json('POST')),
  verify: (id: string, code: string) => request<void>(`${API_BASE}/auth/challenges/${id}/verify`, json('POST', { code })),
  logout: () => request<void>(`${API_BASE}/auth/session`, json('DELETE')),
  logoutAll: (keepCurrent = false) => request<{ revoked: number; kept_current: boolean }>(`${API_BASE}/auth/sessions?keep_current=${keepCurrent}`, json('DELETE')),
}
