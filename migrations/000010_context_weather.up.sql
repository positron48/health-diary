-- City-level places, context periods, daily weather, life_context kind, motivation_score.

ALTER TABLE health_events DROP CONSTRAINT IF EXISTS health_events_kind_check;
ALTER TABLE health_events ADD CONSTRAINT health_events_kind_check CHECK (kind IN (
  'pain_observation',
  'medication_intake',
  'wellbeing',
  'activity',
  'sleep',
  'food_drink',
  'measurement',
  'note',
  'life_context'
));

ALTER TABLE daily_checkins ADD COLUMN IF NOT EXISTS motivation_score smallint;
ALTER TABLE daily_checkins DROP CONSTRAINT IF EXISTS daily_checkins_motivation_score_check;
ALTER TABLE daily_checkins ADD CONSTRAINT daily_checkins_motivation_score_check
  CHECK (motivation_score IS NULL OR (motivation_score >= 0 AND motivation_score <= 10));

CREATE TABLE IF NOT EXISTS places (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  label text NOT NULL,
  region text,
  country_code text NOT NULL DEFAULT 'RU',
  timezone text NOT NULL DEFAULT 'Europe/Moscow',
  provider text NOT NULL DEFAULT 'open-meteo',
  provider_place_id text NOT NULL,
  latitude numeric(9,6) NOT NULL,
  longitude numeric(9,6) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_place_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS places_user_label_uidx
  ON places (user_id, lower(label))
  WHERE user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS context_periods (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  period_type text NOT NULL CHECK (period_type IN ('vacation', 'trip', 'temporary_stay', 'relocation', 'other')),
  place_id uuid REFERENCES places(id) ON DELETE SET NULL,
  place_label text,
  started_on date NOT NULL,
  ended_on date,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed', 'cancelled')),
  source_entry_id uuid REFERENCES journal_entries(id) ON DELETE SET NULL,
  created_from_event_id uuid REFERENCES health_events(id) ON DELETE SET NULL,
  revision int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (ended_on IS NULL OR ended_on >= started_on)
);

CREATE INDEX IF NOT EXISTS context_periods_user_started_idx
  ON context_periods (user_id, started_on DESC);
CREATE INDEX IF NOT EXISTS context_periods_user_open_idx
  ON context_periods (user_id)
  WHERE status = 'open';

CREATE TABLE IF NOT EXISTS daily_weather (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  place_id uuid NOT NULL REFERENCES places(id) ON DELETE CASCADE,
  local_date date NOT NULL,
  provider text NOT NULL DEFAULT 'open-meteo',
  temp_min_c numeric(5,2),
  temp_max_c numeric(5,2),
  temp_mean_c numeric(5,2),
  pressure_mean_hpa numeric(7,2),
  pressure_delta_24h_hpa numeric(7,2),
  humidity_mean_pct numeric(5,2),
  precipitation_mm numeric(7,2),
  weather_code int,
  is_complete boolean NOT NULL DEFAULT true,
  fetched_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (place_id, local_date, provider)
);

CREATE INDEX IF NOT EXISTS daily_weather_user_date_idx
  ON daily_weather (user_id, local_date DESC);

-- Seed default home city (Липецк) as a shared catalog place.
INSERT INTO places (id, user_id, label, region, country_code, timezone, provider, provider_place_id, latitude, longitude)
VALUES (
  '00000000-0000-4000-8000-000000000001',
  NULL,
  'Липецк',
  'Липецкая область',
  'RU',
  'Europe/Moscow',
  'open-meteo',
  'lipetsk',
  52.608800,
  39.599200
)
ON CONFLICT (provider, provider_place_id) DO NOTHING;
