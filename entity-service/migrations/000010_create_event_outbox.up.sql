CREATE TYPE event_outbox_status_enum AS ENUM (
  'waiting',
  'dispatching',
  'dispatched'
);

CREATE TABLE event_outbox (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type    TEXT NOT NULL,
  entity_id     TEXT NOT NULL,
  payload       JSONB NOT NULL,
  status        event_outbox_status_enum NOT NULL DEFAULT 'waiting',
  attempts      INT NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ DEFAULT NOW(),
  claimed_at    TIMESTAMPTZ,
  dispatched_at TIMESTAMPTZ
);

-- Index to optimize the polling fallback's search for waiting rows, oldest first.

CREATE INDEX idx_event_outbox_status_created_at ON event_outbox(status, created_at);
