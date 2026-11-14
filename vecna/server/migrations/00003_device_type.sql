-- +goose Up
ALTER TABLE devices ADD COLUMN device_type TEXT NOT NULL DEFAULT 'light';

-- +goose Down
ALTER TABLE devices DROP COLUMN device_type;
