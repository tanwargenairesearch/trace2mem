CREATE TABLE IF NOT EXISTS blob_deletions (
 key text PRIMARY KEY, tenant text NOT NULL, space text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
