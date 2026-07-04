CREATE TABLE IF NOT EXISTS inflation_rates (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    country_code TEXT NOT NULL, -- World Bank ISO2 (e.g. 'KZ', 'US', 'XC' for euro area)
    year INT NOT NULL,
    rate_percent DOUBLE PRECISION NOT NULL,
    source TEXT NOT NULL DEFAULT 'worldbank',
    UNIQUE (country_code, year)
);
