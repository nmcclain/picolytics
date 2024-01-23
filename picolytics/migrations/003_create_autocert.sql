CREATE TABLE autocert_cache (
    key VARCHAR PRIMARY KEY,
    data BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

---- create above / drop below ----

DROP TABLE autocert_cache;
