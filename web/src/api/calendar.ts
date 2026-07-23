import { API_BASE, request } from './client'
import type { CalendarResponse } from './types'
export type CalendarLayer = 'pain' | 'medication' | 'activity' | 'sleep' | 'wellbeing' | 'context' | 'weather'
export const calendarApi = {
  month: (month: string) => request<CalendarResponse>(`${API_BASE}/calendar?month=${month}`),
}
