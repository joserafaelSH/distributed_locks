-- init.sql
-- Runs automatically when the Postgres container starts, IF mounted into
-- /docker-entrypoint-initdb.d/ (see docker-compose snippet below).
-- NOTE: only runs on a fresh data volume — Postgres skips it if the
-- data directory already has data. Wipe the volume to re-seed.

CREATE TABLE IF NOT EXISTS jobs (
    id            SERIAL PRIMARY KEY,
    payload       JSONB NOT NULL,

    -- lifecycle: pending -> processing -> done
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',

    -- who currently holds the lock (worker id / hostname / pid), if any
    locked_by     VARCHAR(100),

    -- fencing token: strictly increasing, assigned at lock-acquire time.
    -- used to reject stale writes from a worker whose lock already expired.
    lock_token    BIGINT,

    -- lock lease expiry — if now() > locked_until, the lock is considered dead
    -- and any worker may steal it, regardless of 'status'.
    locked_until  TIMESTAMPTZ,

    -- last token that was actually allowed to write a result for this job
    fenced_token  BIGINT NOT NULL DEFAULT 0,

    processed_at  TIMESTAMPTZ,
    result        TEXT,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast lookup for "give me an unlocked pending job"
CREATE INDEX IF NOT EXISTS idx_jobs_status_locked
    ON jobs (status, locked_until);

-- Global fencing-token counter. Every successful lock acquisition pulls
-- the next value from this sequence — guarantees strictly increasing
-- tokens across all workers, no coordination needed.
CREATE SEQUENCE IF NOT EXISTS fencing_token_seq START 1;

-- Seed 1000 sample jobs with varied payloads
INSERT INTO jobs (payload, status)
SELECT
    jsonb_build_object(
        'event_type', (ARRAY['signup', 'payment', 'email_send', 'report', 'cleanup'])[floor(random() * 5 + 1)],
        'user_id', floor(random() * 10000 + 1),
        'amount', round((random() * 500)::numeric, 2)
    ),
    'pending'
FROM generate_series(1, 1000);