-- +goose Up
CREATE TABLE devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    unique_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    state_topic TEXT NOT NULL,
    command_topic TEXT NOT NULL,
    supports_brightness BOOLEAN NOT NULL DEFAULT 0,
    last_power_state TEXT NOT NULL DEFAULT '',
    last_brightness INTEGER NOT NULL DEFAULT 0,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_devices_unique_id ON devices (unique_id);

-- +goose Down
DROP TABLE devices;
