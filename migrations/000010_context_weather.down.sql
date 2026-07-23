DROP TABLE IF EXISTS daily_weather;
DROP TABLE IF EXISTS context_periods;
DROP TABLE IF EXISTS places;

ALTER TABLE daily_checkins DROP CONSTRAINT IF EXISTS daily_checkins_motivation_score_check;
ALTER TABLE daily_checkins DROP COLUMN IF EXISTS motivation_score;

ALTER TABLE health_events DROP CONSTRAINT IF EXISTS health_events_kind_check;
ALTER TABLE health_events ADD CONSTRAINT health_events_kind_check CHECK (kind IN (
  'pain_observation',
  'medication_intake',
  'wellbeing',
  'activity',
  'sleep',
  'food_drink',
  'measurement',
  'note'
));
