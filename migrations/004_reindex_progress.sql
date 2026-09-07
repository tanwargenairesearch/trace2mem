CREATE TABLE reindex_pages (
 tenant text NOT NULL,
 space text NOT NULL,
 epoch bigint NOT NULL,
 path text NOT NULL,
 embedding vector NOT NULL,
 PRIMARY KEY(tenant,space,epoch,path),
 FOREIGN KEY(tenant,space) REFERENCES spaces(tenant,id) ON DELETE CASCADE
);
