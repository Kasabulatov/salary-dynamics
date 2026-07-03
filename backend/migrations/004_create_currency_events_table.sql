CREATE TABLE IF NOT EXISTS currency_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    date DATE NOT NULL,
    base_currency TEXT NOT NULL,
    quote_currency TEXT NOT NULL,
    change_window TEXT NOT NULL, -- 'daily' | 'weekly'
    percent_change DOUBLE PRECISION NOT NULL,
    absolute_change DOUBLE PRECISION NOT NULL,
    news_headline TEXT, -- v3
    news_url TEXT,      -- v3
    UNIQUE (date, base_currency, quote_currency, change_window)
);

CREATE INDEX IF NOT EXISTS idx_currency_events_lookup
    ON currency_events (base_currency, quote_currency, date);
