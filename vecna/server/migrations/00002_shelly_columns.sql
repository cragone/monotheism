-- +goose Up
ALTER TABLE devices DROP COLUMN state_topic;
ALTER TABLE devices DROP COLUMN command_topic;
ALTER TABLE devices ADD COLUMN host TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN switch_id INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE devices DROP COLUMN host;
ALTER TABLE devices DROP COLUMN switch_id;
ALTER TABLE devices ADD COLUMN state_topic TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN command_topic TEXT NOT NULL DEFAULT '';
