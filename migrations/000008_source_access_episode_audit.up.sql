CREATE TABLE source_entry_access_audits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_id uuid NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    accessed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX source_entry_access_audits_user_time_idx
    ON source_entry_access_audits(user_id, accessed_at DESC);

CREATE TABLE episode_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id uuid NOT NULL REFERENCES symptom_episodes(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision int NOT NULL,
    action text NOT NULL CHECK (action IN ('close','reopen','projection')),
    before_data jsonb NOT NULL,
    after_data jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(episode_id, revision)
);
ALTER TABLE symptom_episodes ADD COLUMN revision int NOT NULL DEFAULT 1;
