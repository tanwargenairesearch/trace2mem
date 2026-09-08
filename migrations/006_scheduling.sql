ALTER TABLE spaces ADD COLUMN compilation_mode text NOT NULL DEFAULT 'automatic' CHECK(compilation_mode IN ('automatic','daily','manual'));
ALTER TABLE spaces ADD COLUMN compilation_time text NOT NULL DEFAULT '02:00';
ALTER TABLE spaces ADD COLUMN compilation_timezone text NOT NULL DEFAULT 'UTC';
ALTER TABLE jobs ADD COLUMN target_watermark bigint NOT NULL DEFAULT 0;
UPDATE jobs j SET target_watermark=COALESCE((SELECT max(ordinal) FROM events e WHERE e.tenant=j.tenant AND e.space=j.space),0);
CREATE TABLE compilation_requests (
 tenant text NOT NULL, memory text NOT NULL, first_received timestamptz NOT NULL,
 due_at timestamptz NOT NULL, watermark bigint NOT NULL,
 PRIMARY KEY(tenant,memory), FOREIGN KEY(tenant,memory) REFERENCES spaces(tenant,id)
);
CREATE INDEX compilation_requests_due ON compilation_requests(due_at);
