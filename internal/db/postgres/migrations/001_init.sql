-- Exchange Database Schema v1
-- ============================================================

-- Users
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    kyc_level       SMALLINT NOT NULL DEFAULT 0,
    kyc_data        JSONB,
    two_fa_secret   VARCHAR(64),
    two_fa_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    anti_phishing   VARCHAR(50),
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

-- Accounts / Balances
CREATE TABLE IF NOT EXISTS accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    currency        VARCHAR(20) NOT NULL,
    balance         NUMERIC(36,18) NOT NULL DEFAULT 0,
    frozen_balance          NUMERIC(36,18) NOT NULL DEFAULT 0,
    version         BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);

-- API Keys
CREATE TABLE IF NOT EXISTS api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    label           VARCHAR(100) NOT NULL,
    api_key         VARCHAR(64) UNIQUE NOT NULL,
    secret_hash     VARCHAR(255) NOT NULL,
    permissions     JSONB NOT NULL DEFAULT '["read"]',
    ip_whitelist    TEXT[] DEFAULT '{}',
    rate_limit      INTEGER DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_api_key ON api_keys(api_key);

-- Markets / Trading Pairs
CREATE TABLE IF NOT EXISTS markets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol          VARCHAR(20) UNIQUE NOT NULL,
    base_currency   VARCHAR(20) NOT NULL,
    quote_currency  VARCHAR(20) NOT NULL,
    price_tick      NUMERIC(18,10) NOT NULL,
    qty_step        NUMERIC(18,10) NOT NULL,
    min_order_qty   NUMERIC(36,18) NOT NULL,
    max_order_qty   NUMERIC(36,18) NOT NULL,
    min_notional    NUMERIC(36,18) NOT NULL DEFAULT 0,
    maker_fee       NUMERIC(5,4) NOT NULL DEFAULT 0.0010,
    taker_fee       NUMERIC(5,4) NOT NULL DEFAULT 0.0010,
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Orders
CREATE TABLE IF NOT EXISTS orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        VARCHAR(50) UNIQUE NOT NULL,
    client_order_id VARCHAR(50),
    user_id         UUID NOT NULL REFERENCES users(id),
    symbol          VARCHAR(20) NOT NULL,
    side            VARCHAR(4) NOT NULL,
    type            VARCHAR(20) NOT NULL,
    time_in_force   VARCHAR(3) NOT NULL DEFAULT 'GTC',
    price           NUMERIC(36,18),
    stop_price      NUMERIC(36,18),
    quantity        NUMERIC(36,18) NOT NULL,
    filled_qty      NUMERIC(36,18) NOT NULL DEFAULT 0,
    status          VARCHAR(20) NOT NULL DEFAULT 'open',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    filled_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_symbol_status ON orders(symbol, status);
CREATE INDEX IF NOT EXISTS idx_orders_client_id ON orders(client_order_id);

-- Trades (Fills)
CREATE TABLE IF NOT EXISTS trades (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trade_id        VARCHAR(50) UNIQUE NOT NULL,
    symbol          VARCHAR(20) NOT NULL,
    maker_order_id  VARCHAR(50) NOT NULL,
    taker_order_id  VARCHAR(50) NOT NULL,
    maker_user_id   UUID NOT NULL,
    taker_user_id   UUID NOT NULL,
    price           NUMERIC(36,18) NOT NULL,
    quantity        NUMERIC(36,18) NOT NULL,
    quote_quantity  NUMERIC(36,18) NOT NULL,
    maker_fee       NUMERIC(36,18) NOT NULL DEFAULT 0,
    taker_fee       NUMERIC(36,18) NOT NULL DEFAULT 0,
    side            VARCHAR(4) NOT NULL,
    executed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trades_symbol_time ON trades(symbol, executed_at DESC);
CREATE INDEX IF NOT EXISTS idx_trades_executed_at ON trades(executed_at);

-- Deposits
CREATE TABLE IF NOT EXISTS deposit_addresses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    currency        VARCHAR(20) NOT NULL,
    chain           VARCHAR(20) NOT NULL,
    address         VARCHAR(42) NOT NULL,
    derivation_path VARCHAR(50) NOT NULL,
    wallet_type     VARCHAR(10) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(address, chain)
);

CREATE TABLE IF NOT EXISTS deposits (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tx_hash         VARCHAR(66) NOT NULL,
    currency        VARCHAR(20) NOT NULL,
    chain           VARCHAR(20) NOT NULL,
    from_address    VARCHAR(42) NOT NULL,
    to_address      VARCHAR(42) NOT NULL,
    amount          NUMERIC(36,18) NOT NULL,
    block_number    BIGINT NOT NULL,
    confirmations   INTEGER NOT NULL DEFAULT 0,
    required_confs  INTEGER NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    user_id         UUID NOT NULL REFERENCES users(id),
    credited_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tx_hash, to_address)
);

CREATE INDEX IF NOT EXISTS idx_deposits_user ON deposits(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deposits_status ON deposits(status);

-- Withdrawals
CREATE TABLE IF NOT EXISTS withdrawals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    withdrawal_id   VARCHAR(50) UNIQUE NOT NULL,
    user_id         UUID NOT NULL REFERENCES users(id),
    currency        VARCHAR(20) NOT NULL,
    chain           VARCHAR(20) NOT NULL,
    to_address      VARCHAR(42) NOT NULL,
    amount          NUMERIC(36,18) NOT NULL,
    fee             NUMERIC(36,18) NOT NULL,
    tx_hash         VARCHAR(66),
    wallet_type     VARCHAR(10) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    approved_by     UUID REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    failure_reason  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_user ON withdrawals(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_withdrawals_status ON withdrawals(status);

-- Balance Transaction Audit Trail
CREATE TABLE IF NOT EXISTS balance_transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    currency        VARCHAR(20) NOT NULL,
    type            VARCHAR(30) NOT NULL,
    amount          NUMERIC(36,18) NOT NULL,
    balance_before  NUMERIC(36,18) NOT NULL,
    balance_after   NUMERIC(36,18) NOT NULL,
    reference_id    VARCHAR(100),
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bal_tx_user ON balance_transactions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bal_tx_ref ON balance_transactions(reference_id);

-- Fee Tiers
CREATE TABLE IF NOT EXISTS fee_tiers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tier_name       VARCHAR(50) NOT NULL UNIQUE,
    min_volume_usd  NUMERIC(20,2) NOT NULL,
    maker_fee       NUMERIC(5,4) NOT NULL,
    taker_fee       NUMERIC(5,4) NOT NULL
);

CREATE TABLE IF NOT EXISTS user_volume_30d (
    user_id         UUID PRIMARY KEY REFERENCES users(id),
    volume_usd      NUMERIC(20,2) NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default fee tiers
INSERT INTO fee_tiers (tier_name, min_volume_usd, maker_fee, taker_fee) VALUES
    ('Tier 0', 0,        0.0010, 0.0010),
    ('Tier 1', 50000,    0.0008, 0.0009),
    ('Tier 2', 500000,   0.0005, 0.00075),
    ('Tier 3', 5000000,  0.0002, 0.0006),
    ('Tier 4', 50000000, 0.0000, 0.0005)
ON CONFLICT DO NOTHING;

-- Seed a test market
INSERT INTO markets (symbol, base_currency, quote_currency, price_tick, qty_step, min_order_qty, max_order_qty, min_notional, maker_fee, taker_fee) VALUES
    ('ETH-USDT', 'ETH', 'USDT', 0.01, 0.0001, 0.001, 1000, 10, 0.0010, 0.0010)
ON CONFLICT DO NOTHING;
