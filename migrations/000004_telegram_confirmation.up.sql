CREATE TABLE telegram_callback_actions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE,
    user_id uuid NOT NULL REFERENCES users(id),
    batch_id uuid NOT NULL REFERENCES event_batches(id),
    batch_version int NOT NULL CHECK (batch_version > 0),
    action text NOT NULL CHECK (action IN ('confirm','reject')),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX telegram_callback_actions_lookup_idx ON telegram_callback_actions(token_hash) WHERE used_at IS NULL;
