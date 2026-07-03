CREATE TABLE IF NOT EXISTS daily_rates (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    date DATE NOT NULL,
    base_currency TEXT NOT NULL,
    quote_currency TEXT NOT NULL,
    rate NUMERIC(18, 8) NOT NULL CHECK (rate > 0),
    source TEXT NOT NULL,
    UNIQUE (date, base_currency, quote_currency)
);

CREATE INDEX IF NOT EXISTS idx_daily_rates_lookup
    ON daily_rates (base_currency, quote_currency, date DESC);
