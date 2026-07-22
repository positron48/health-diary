UPDATE users
SET timezone = 'Europe/Moscow', updated_at = now()
WHERE btrim(timezone) = '';
