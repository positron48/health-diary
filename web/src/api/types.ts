export type EventKind = 'pain' | 'pain_observation' | 'medication_intake' | 'wellbeing' | 'activity' | 'sleep' | 'food_drink' | 'measurement' | 'note' | 'life_context'
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
export interface ProcessingEntry {
  id: string
  source_type: string
  source_sent_at: string
  processing_status: 'queued' | 'processing' | 'failed' | string
}
export interface InboxResponse {
  processing: ProcessingEntry[]
  batches: PendingBatch[]
  processing_count?: number
  batch_count?: number
  count: number
}
export interface UserSettings { day_start_time?: string; home_place_id?: string }
export type ContextPeriodType = 'vacation' | 'trip' | 'temporary_stay' | 'relocation' | 'other'
export interface PlaceCandidate {
  provider_place_id: string
  label: string
  region?: string
  country_code: string
  timezone: string
  latitude: number
  longitude: number
}
export interface ActiveLocation {
  place_id: string
  place_label: string
  source: 'home' | 'period'
  period_type?: ContextPeriodType
  local_date: string
}
export interface ContextPeriod {
  id: string
  period_type: ContextPeriodType
  place_id?: string
  place_label: string
  started_on: string
  ended_on?: string
  status: 'open' | 'closed'
  revision: number
}
export interface ContextPeriodsResponse { active: ActiveLocation; periods: ContextPeriod[] }
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
  date: string
  has_data: boolean
  has_health_data?: boolean
  has_pending?: boolean
  pending_count?: number
  pain?: { max_intensity?: number | null; open?: boolean; episodes?: number }
  medication?: { intakes: number }
  activity?: { minutes: number | null }
  sleep?: { minutes: number | null; quality: number | null }
  wellbeing?: { score?: number | null; motivation?: number | null; energy?: number | null }
  context?: { period_type: string; place_label?: string; segment?: 'start' | 'middle' | 'end' }
  weather?: {
    temp_min_c?: number | null; temp_max_c?: number | null; temp_mean_c?: number | null; weather_code?: number | null
    pressure_mean_hpa?: number | null; pressure_delta_24h_hpa?: number | null; humidity_mean_pct?: number | null
    precipitation_mm?: number | null; is_complete?: boolean
  }
}
export interface CalendarResponse {
  month: string
  timezone: string
  days: CalendarDay[]
  layers_available?: string[]
  mode?: string
}
export interface DayResponse { date: string; events: HealthEvent[]; pending_count?: number; episodes?: Episode[] }
export interface Episode { id: string; status: 'open' | 'closed'; started_at: string; ended_at?: string | null; time_precision?: Precision; duration_minutes?: number | null; max_intensity?: number | null; observation_count?: number; events?: HealthEvent[]; revision: number }
export interface AnalyticsSummary {
  timezone?: string; observation_days?: number; diary_days?: number; confirmed_events?: number; pending_events?: number
  headache_days?: number; medication_days?: number; closed_episode_count?: number; total_episode_count?: number
  pain?: Record<string, number | null>; medication?: Record<string, number | null>; sleep?: Record<string, number | null>
  activity?: Record<string, number | null>; wellbeing?: Record<string, number | null>
  associations?: Array<{ label: string; sample_size: number; description: string }>; formula_version?: string; limitations?: string[]
  weather?: WeatherAnalytics
}
export interface WeatherDayPoint {
  date: string; temp_mean_c?: number | null; pressure_mean_hpa?: number | null; pressure_delta_24h_hpa?: number | null
  humidity_mean_pct?: number | null; precipitation_mm?: number | null; is_complete: boolean; pain_starts: number
}
export interface PainWeatherWindow {
  occurred_at: string; pressure_hpa?: number | null; pressure_delta_6h_hpa?: number | null; pressure_delta_24h_hpa?: number | null; intensity?: number | null
}
export interface WeatherFactor {
  key: string; label: string; status: 'insufficient_data' | 'possible_association' | 'no_increased_rate' | string
  exposed_days: number; unexposed_days: number; exposed_start_days: number; unexposed_start_days: number
  exposed_rate?: number | null; unexposed_rate?: number | null; risk_ratio?: number | null; description: string
  requirements: Record<string, { actual: number; required: number }>
}
export interface WeatherAnalytics {
  status: string; coverage: { weather_days: number; pressure_days: number; headache_start_days: number; headache_starts: number }
  series: WeatherDayPoint[]; pain_windows: PainWeatherWindow[]; factors: WeatherFactor[]; formula_version: string; limitation: string
}
export interface ApiErrorBody { error?: { code?: string; message?: string; fields?: Record<string, string>; request_id?: string } }
export interface SourceEntry { id: string; source_type: string; source_sent_at: string; text: string }
