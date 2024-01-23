-- name: GetEvent :one
SELECT * FROM events
WHERE id = $1 LIMIT 1;

-- name: ListEvents :many
SELECT * FROM events
ORDER BY id DESC;

-- name: UpsertDomain :one
INSERT INTO domains (domain_name)
VALUES ($1)
ON CONFLICT (domain_name) DO UPDATE
SET domain_name = EXCLUDED.domain_name
RETURNING domain_id;

-- name: GetSession :one
SELECT id FROM sessions
WHERE visitor_id = $1
AND updated_at > CURRENT_TIMESTAMP - $2::interval
ORDER by updated_at DESC
LIMIT 1;

---- MUST call this in a transaction after GetSession ----
-- name: CreateSession :one
INSERT INTO sessions (
    updated_at, bounce, domain_id, exit_path, ---- values updated with each event ----
    visitor_id, entry_path, ---- static values from first event ---- 
    country, latitude, longitude, subdivision, city, ---- geoip lookup ----
    browser, browser_version, os, os_version, platform, device_type, bot, screen_w, screen_h, timezone, pixel_ratio, pixel_depth, ---- useragent ----
    utm_source, utm_medium, utm_campaign, utm_content, utm_term ---- query string ----
) VALUES (
  CURRENT_TIMESTAMP, TRUE, $1, $2,
  $3, $4,
  $5, $6, $7, $8, $9,
  $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
  $22, $23, $24, $25, $26
)
RETURNING id;

---- MUST call this in a transaction after GetSession ----
-- name: UpdateSession :exec
UPDATE sessions 
SET 
    bounce = CASE WHEN @event_name::text NOT IN ('hidden', 'ping') THEN FALSE ELSE bounce END,
    updated_at = CURRENT_TIMESTAMP,
    exit_path = $1,
    duration = EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - created_at))
WHERE id = $2;

-- name: CreateEvents :copyfrom
INSERT INTO events (
  domain_id, session_id, visitor_id, name, path, referrer,
  load_time, ttfb
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: PruneSessions :exec
DELETE FROM sessions WHERE updated_at <= CURRENT_TIMESTAMP - @the_interval::interval;

-- name: PruneEvents :exec
DELETE FROM events WHERE created_at <= CURRENT_TIMESTAMP - @the_interval::interval;

-- name: UpdateSalt :exec
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM salt) OR (SELECT EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - created_at)) / 3600 > 0 FROM salt LIMIT 1) THEN
        DELETE FROM salt;
        INSERT INTO salt (salt) VALUES (gen_random_uuid());
    END IF;
END $$;

-- name: GetSalt :one
SELECT salt, created_at FROM salt LIMIT 1;

-- name: AutocertCachePut :exec
INSERT INTO autocert_cache (key, data, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (key)
DO UPDATE SET data = EXCLUDED.data, updated_at = NOW();

-- name: AutocertCacheGet :one
SELECT data FROM autocert_cache WHERE key = $1;

-- name: AutocertCacheDelete :exec
DELETE FROM autocert_cache WHERE key = $1;
