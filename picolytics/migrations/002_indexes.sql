CREATE INDEX idx_sessions_created_at ON sessions(created_at);
CREATE INDEX idx_sessions_domain_id ON sessions(domain_id);
CREATE INDEX idx_sessions_visitor_id ON sessions(visitor_id);

CREATE INDEX idx_events_created_at ON events(created_at);
CREATE INDEX idx_events_domain_id ON events(domain_id);
CREATE INDEX idx_events_session_id ON events(session_id);
CREATE INDEX idx_events_visitor_id ON events(visitor_id);

---- create above / drop below ----
DROP INDEX idx_sessions_created_at;
DROP INDEX idx_sessions_domain_id;
DROP INDEX idx_sessions_visitor_id;

DROP INDEX idx_events_created_at;
DROP INDEX idx_events_domain_id;
DROP INDEX idx_events_session_id;
DROP INDEX idx_events_visitor_id;
