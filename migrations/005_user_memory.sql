CREATE TABLE user_memories (
 tenant text NOT NULL, subject text NOT NULL, memory_id text NOT NULL,
 PRIMARY KEY (tenant, subject), UNIQUE (tenant, memory_id),
 FOREIGN KEY (tenant, memory_id) REFERENCES spaces(tenant,id)
);
