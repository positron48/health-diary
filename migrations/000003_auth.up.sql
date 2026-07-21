CREATE TABLE auth_challenges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    public_token_hash bytea NOT NULL UNIQUE,
    user_id uuid REFERENCES users(id),
    code_hash bytea,
    expires_at timestamptz NOT NULL,
    attempt_count int NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts int NOT NULL CHECK (max_attempts > 0),
    bound_at timestamptz,
    consumed_at timestamptz,
    locked_at timestamptz,
    request_ip_hash bytea,
    user_agent_hash bytea,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_challenges_expiry_idx ON auth_challenges(expires_at);

CREATE TABLE web_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    ip_hash bytea,
    user_agent_hash bytea,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX web_sessions_expiry_idx ON web_sessions(expires_at);
