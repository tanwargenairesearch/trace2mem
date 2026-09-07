CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE IF NOT EXISTS spaces (
 tenant text NOT NULL, id text NOT NULL, name text NOT NULL,
 revision text NOT NULL DEFAULT '', watermark bigint NOT NULL DEFAULT 0,
 generation bigint NOT NULL DEFAULT 0, suppressed boolean NOT NULL DEFAULT false,
 model jsonb NOT NULL DEFAULT '{}', credential bytea,
 PRIMARY KEY(tenant,id)
);
CREATE TABLE IF NOT EXISTS members (
 tenant text NOT NULL, space text NOT NULL, subject text NOT NULL, role text NOT NULL CHECK(role IN ('owner','editor','reader')),
 PRIMARY KEY(tenant,space,subject), FOREIGN KEY(tenant,space) REFERENCES spaces(tenant,id)
);
CREATE TABLE IF NOT EXISTS tokens (
 hash text PRIMARY KEY, tenant text NOT NULL, subject text NOT NULL, scopes text[] NOT NULL,
 expires_at timestamptz NOT NULL, revoked boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS events (
 ordinal bigserial UNIQUE, tenant text NOT NULL, space text NOT NULL, id text NOT NULL,
 session text NOT NULL, hash text NOT NULL, payload jsonb, occurred_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(), deleted boolean NOT NULL DEFAULT false,
 PRIMARY KEY(tenant,space,id), FOREIGN KEY(tenant,space) REFERENCES spaces(tenant,id)
);
CREATE INDEX IF NOT EXISTS events_space_ordinal ON events(tenant,space,ordinal);
CREATE TABLE IF NOT EXISTS jobs (
 tenant text NOT NULL, space text NOT NULL, status text NOT NULL DEFAULT 'pending',
 fence bigint NOT NULL DEFAULT 0, lease_until timestamptz, requested boolean NOT NULL DEFAULT true,
 attempts integer NOT NULL DEFAULT 0, error text NOT NULL DEFAULT '', available_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant,space), FOREIGN KEY(tenant,space) REFERENCES spaces(tenant,id)
);
CREATE TABLE IF NOT EXISTS revisions (
 tenant text NOT NULL, space text NOT NULL, id text NOT NULL, parent text NOT NULL,
 watermark bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), verification jsonb NOT NULL,
 PRIMARY KEY(tenant,space,id), FOREIGN KEY(tenant,space) REFERENCES spaces(tenant,id)
);
CREATE TABLE IF NOT EXISTS pages (
 tenant text NOT NULL, space text NOT NULL, revision text NOT NULL, path text NOT NULL,
 content text NOT NULL, hash text NOT NULL, citations text[] NOT NULL, embedding vector, embedding_model text NOT NULL DEFAULT '',
 PRIMARY KEY(tenant,space,revision,path), FOREIGN KEY(tenant,space,revision) REFERENCES revisions(tenant,space,id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS artifacts (
 tenant text NOT NULL, space text NOT NULL, id text NOT NULL, key text NOT NULL, hash text NOT NULL, size bigint NOT NULL, media_type text NOT NULL,
 PRIMARY KEY(tenant,space,id), FOREIGN KEY(tenant,space) REFERENCES spaces(tenant,id)
);
CREATE TABLE IF NOT EXISTS deletions (
 tenant text NOT NULL, space text NOT NULL, event_id text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant,space,event_id)
);
CREATE TABLE IF NOT EXISTS usage (
 id bigserial PRIMARY KEY, tenant text NOT NULL, space text NOT NULL, operation text NOT NULL,
 input_tokens bigint NOT NULL, output_tokens bigint NOT NULL, estimated boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS proposals (
 id text PRIMARY KEY, tenant text NOT NULL, space text NOT NULL, parent text NOT NULL,
 status text NOT NULL, content jsonb NOT NULL, verification jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS evaluations (
 id text PRIMARY KEY, tenant text NOT NULL, space text NOT NULL, revision text NOT NULL,
 report jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS candidates (
 id text PRIMARY KEY, tenant text NOT NULL, space text NOT NULL, prompt text NOT NULL,
 evaluation_id text NOT NULL, promoted boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS audit (
 id bigserial PRIMARY KEY, tenant text NOT NULL, subject text NOT NULL, space text NOT NULL,
 action text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
