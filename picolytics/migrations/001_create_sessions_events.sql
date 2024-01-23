CREATE TABLE domains (
    domain_id SERIAL PRIMARY KEY,
    domain_name TEXT UNIQUE NOT NULL
);

CREATE TABLE sessions (
    id BIGSERIAL PRIMARY KEY,
    ---- values updated with each event: ----
    updated_at TIMESTAMPTZ,
    duration INT DEFAULT 0,
    bounce BOOLEAN NOT NULL DEFAULT FALSE,
    exit_path TEXT NOT NULL DEFAULT '',
    ---- static values from first event: ----
    visitor_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    domain_id INT NOT NULL,
    entry_path TEXT NOT NULL,
    ---- geoip lookup: ----
    country VARCHAR(4),
    latitude FLOAT,
    longitude FLOAT,
    subdivision TEXT,
    city TEXT,
    ---- useragent: ----
    browser TEXT,
    browser_version TEXT,
    os TEXT,
    os_version TEXT,
    platform TEXT,
    device_type TEXT,
    bot BOOLEAN NOT NULL DEFAULT true,
    screen_w INT,
    screen_h INT,
    timezone TEXT,
    pixel_ratio FLOAT,
    pixel_depth INT,
    ---- query string args via javascript: ----
	utm_source   TEXT,
	utm_medium   TEXT,
	utm_campaign TEXT,
	utm_content  TEXT,
	utm_term     TEXT,
    ---- : ----
    FOREIGN KEY (domain_id) REFERENCES domains(domain_id)
);

CREATE TABLE events (
    id BIGSERIAL PRIMARY KEY,
    ---- event essentials ----
    name TEXT NOT NULL,
    domain_id INT NOT NULL,
    path TEXT NOT NULL,
    referrer TEXT NOT NULL,
    visitor_id TEXT NOT NULL,
    session_id BIGINT NOT NULL,
    ---- timing ----
    load_time INT NOT NULL DEFAULT 0,
    ttfb INT NOT NULL DEFAULT 0,
    ---- : ----
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES domains(domain_id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE salt (
    salt UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

---- create above / drop below ----

DROP TABLE events;
DROP TABLE sessions;
DROP TABLE domains;
DROP TABLE salt;

