CREATE TABLE deletion_audits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    requested_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    status text NOT NULL CHECK (status IN ('queued','completed','failed')),
    error_code text
);
