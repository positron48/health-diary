import { API_BASE, request } from './client'
import type { CalendarResponse } from './types'
export type CalendarMode = 'overview' | 'pain' | 'medication' | 'activity' | 'sleep' | 'wellbeing'
export const calendarApi = { month: (month: string, mode: CalendarMode) => request<CalendarResponse>(`${API_BASE}/calendar?month=${month}&mode=${mode}`) }
