CREATE TABLE IF NOT EXISTS readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recorded_at TEXT NOT NULL,
    co2_ppm INTEGER NULL,
    voc_ppb INTEGER NULL,
    temperature_c REAL NULL,
    humidity_percent REAL NULL,
    pressure_hpa REAL NULL,
    radon_short_bqm3 INTEGER NULL,
    radon_long_bqm3 INTEGER NULL,
    raw_payload BLOB NULL
);

CREATE INDEX IF NOT EXISTS idx_readings_recorded_at ON readings(recorded_at);

CREATE INDEX IF NOT EXISTS idx_readings_co2_time ON readings(recorded_at) WHERE co2_ppm IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_voc_time ON readings(recorded_at) WHERE voc_ppb IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_temperature_time ON readings(recorded_at) WHERE temperature_c IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_humidity_time ON readings(recorded_at) WHERE humidity_percent IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_pressure_time ON readings(recorded_at) WHERE pressure_hpa IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_radon_short_time ON readings(recorded_at) WHERE radon_short_bqm3 IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_radon_long_time ON readings(recorded_at) WHERE radon_long_bqm3 IS NOT NULL;
