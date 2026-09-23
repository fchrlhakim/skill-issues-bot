CREATE TABLE IF NOT EXISTS guild_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discord_id TEXT NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    tier TEXT NOT NULL,
    account_age_days INTEGER NOT NULL DEFAULT 0,
    restricted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_guild_members_discord_id ON guild_members (discord_id);
CREATE INDEX IF NOT EXISTS idx_guild_members_tier ON guild_members (tier);

CREATE TABLE IF NOT EXISTS ticket_counters (
    id INTEGER PRIMARY KEY,
    seq INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO ticket_counters (id, seq) VALUES (1, 0) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    type TEXT NOT NULL,
    code TEXT NOT NULL,
    region TEXT NOT NULL,
    workflow TEXT NOT NULL,
    status TEXT NOT NULL,
    opener_id TEXT NOT NULL,
    opener_tag TEXT NOT NULL DEFAULT '',
    thread_id TEXT NOT NULL,
    assigned_to TEXT NULL,
    summary_message_id TEXT NOT NULL DEFAULT '',
    opened_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ NULL,
    data_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    history_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    resolution_json JSONB NULL,
    withdrawal_json JSONB NULL,
    seller_approval_json JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_tickets_public_id ON tickets (public_id);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_tickets_thread_id ON tickets (thread_id);
CREATE INDEX IF NOT EXISTS idx_tickets_opener_status ON tickets (opener_id, status);
CREATE INDEX IF NOT EXISTS idx_tickets_status_opened ON tickets (status, opened_at);
CREATE INDEX IF NOT EXISTS idx_tickets_assigned ON tickets (assigned_to, opened_at);

CREATE TABLE IF NOT EXISTS outgoing_mutations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id TEXT NOT NULL,
    ticket_public_id TEXT NOT NULL,
    seller_id TEXT NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency TEXT NOT NULL,
    reference TEXT NOT NULL,
    by TEXT NOT NULL,
    paid_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_outgoing_public_id ON outgoing_mutations (public_id);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_outgoing_reference ON outgoing_mutations (reference);
CREATE INDEX IF NOT EXISTS idx_outgoing_seller_paid ON outgoing_mutations (seller_id, paid_at DESC);
