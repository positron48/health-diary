CREATE TABLE IF NOT EXISTS hourly_weather (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  place_id uuid NOT NULL REFERENCES places(id) ON DELETE CASCADE,
  observed_at timestamptz NOT NULL,
  local_date date NOT NULL,
  provider text NOT NULL DEFAULT 'open-meteo',
  temperature_c numeric(5,2),
  pressure_msl_hpa numeric(7,2),
  humidity_pct numeric(5,2),
  precipitation_mm numeric(7,2),
  is_complete boolean NOT NULL DEFAULT true,
  fetched_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, place_id, observed_at, provider)
);

CREATE INDEX IF NOT EXISTS hourly_weather_user_date_idx
  ON hourly_weather (user_id, local_date, observed_at);

CREATE INDEX IF NOT EXISTS hourly_weather_place_observed_idx
  ON hourly_weather (place_id, observed_at)
  WHERE pressure_msl_hpa IS NOT NULL;
