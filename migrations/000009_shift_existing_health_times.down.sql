UPDATE health_events
SET occurred_at = occurred_at + interval '3 hours',
    ended_at = CASE WHEN ended_at IS NULL THEN NULL ELSE ended_at + interval '3 hours' END;

UPDATE symptom_episodes
SET started_at = started_at + interval '3 hours',
    ended_at = CASE WHEN ended_at IS NULL THEN NULL ELSE ended_at + interval '3 hours' END;

UPDATE medication_intakes
SET effect_observed_at = effect_observed_at + interval '3 hours'
WHERE effect_observed_at IS NOT NULL;

UPDATE sleep_records
SET sleep_started_at = sleep_started_at + interval '3 hours',
    sleep_ended_at = CASE WHEN sleep_ended_at IS NULL THEN NULL ELSE sleep_ended_at + interval '3 hours' END
WHERE sleep_started_at IS NOT NULL OR sleep_ended_at IS NOT NULL;
