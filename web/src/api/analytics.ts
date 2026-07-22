import { API_BASE, request } from './client'
import type { AnalyticsSummary } from './types'

interface SummaryResponse {
  coverage: { observation_days: number; diary_days: number; confirmed_events: number; pending_events: number; episodes: number; closed_episodes: number }
  metrics: AnalyticsSummary & {
    pain_intensity_known?: number; pain_intensity_average?: number | null; pain_intensity_maximum?: number | null
    medication_intakes?: number; recorded_medication_effects?: number; sleep_minutes?: number; sleep_records?: number
    activity_minutes?: number; activity_records?: number; wellbeing_records?: number; wellbeing_score_known?: number
  }
  formula_version: string
}
export const analyticsApi = {
  async summary(days: number): Promise<AnalyticsSummary> {
    const to = new Date(), from = new Date(to)
    from.setDate(from.getDate() - days + 1)
    const date = (value: Date) => value.toISOString().slice(0, 10)
    const response = await request<SummaryResponse>(`${API_BASE}/analytics/summary?from=${date(from)}&to=${date(to)}`)
    const metrics = response.metrics
    return {
      ...metrics,
      ...response.coverage,
      total_episode_count: response.coverage.episodes,
      closed_episode_count: response.coverage.closed_episodes,
      formula_version: response.formula_version,
      pain: { days: metrics.headache_days ?? 0, known_intensity_n: metrics.pain_intensity_known ?? 0, average_intensity: metrics.pain_intensity_average ?? null, maximum_intensity: metrics.pain_intensity_maximum ?? null },
      medication: { days: metrics.medication_days ?? 0, intakes: metrics.medication_intakes ?? 0, recorded_effect_n: metrics.recorded_medication_effects ?? 0 },
      sleep: { minutes: metrics.sleep_minutes ?? 0, records: metrics.sleep_records ?? 0 },
      activity: { minutes: metrics.activity_minutes ?? 0, records: metrics.activity_records ?? 0 },
      wellbeing: { records: metrics.wellbeing_records ?? 0, score_known_n: metrics.wellbeing_score_known ?? 0 },
    }
  },
}
