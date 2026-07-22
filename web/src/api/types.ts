export type EventKind = 'pain' | 'pain_observation' | 'medication_intake' | 'wellbeing' | 'activity' | 'sleep' | 'food_drink' | 'measurement' | 'note'
export type Precision = 'exact' | 'approximate' | 'unknown' | 'date_only' | 'inferred_from_message'

export interface HealthEvent {
  id: string
  kind: EventKind | string
  occurred_at: string
  ended_at?: string | null
  time_precision?: Precision
  data?: Record<string, unknown>
  attributes?: Record<string, unknown>
  revision: number
  episode_id?: string | null
  entry_id?: string
  source_entry_id?: string
}
export interface PendingBatch {
  id: string
  version: number
  created_at: string
  entry_id?: string
  source?: string
  source_type?: string
  message_at?: string
  events: HealthEvent[]
}
export interface UserSettings { day_start_time?: string }
export interface Session {
  id: string
  timezone: string
  locale?: string
  settings?: UserSettings
  current_local_date?: string
  display_name?: string
  expires_at?: string
}
export interface CalendarDay {
  date: string; has_data: boolean; pending_count?: number
  pain?: { episodes: number; max_intensity: number | null; open?: boolean }
  medication?: { intakes: number }; activity?: { minutes: number | null }
  sleep?: { minutes: number | null; quality: number | null }; wellbeing?: { score: number | null }
}
export interface CalendarResponse { month: string; timezone: string; days: CalendarDay[]; mode?: string }
export interface DayResponse { date: string; events: HealthEvent[]; pending_count?: number; episodes?: Episode[] }
export interface Episode { id: string; status: 'open' | 'closed'; started_at: string; ended_at?: string | null; time_precision?: Precision; duration_minutes?: number | null; max_intensity?: number | null; observation_count?: number; events?: HealthEvent[]; revision: number }
export interface AnalyticsSummary {
  timezone?: string; observation_days?: number; diary_days?: number; confirmed_events?: number; pending_events?: number
  headache_days?: number; medication_days?: number; closed_episode_count?: number; total_episode_count?: number
  pain?: Record<string, number | null>; medication?: Record<string, number | null>; sleep?: Record<string, number | null>
  activity?: Record<string, number | null>; wellbeing?: Record<string, number | null>
  associations?: Array<{ label: string; sample_size: number; description: string }>; formula_version?: string; limitations?: string[]
}
export interface ApiErrorBody { error?: { code?: string; message?: string; fields?: Record<string, string>; request_id?: string } }
export interface SourceEntry { id: string; source_type: string; source_sent_at: string; text: string }
