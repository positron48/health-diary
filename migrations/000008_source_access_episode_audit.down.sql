ALTER TABLE symptom_episodes DROP COLUMN IF EXISTS revision;
DROP TABLE IF EXISTS episode_revisions;
DROP TABLE IF EXISTS source_entry_access_audits;
