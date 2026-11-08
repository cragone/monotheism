-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_users_username ON users (username);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions (user_id);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

CREATE TABLE threads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users (id),
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_threads_activity ON threads (last_activity DESC);
CREATE INDEX idx_threads_created ON threads (created_at DESC);

CREATE TABLE replies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id INTEGER NOT NULL REFERENCES threads (id),
    parent_id INTEGER REFERENCES replies (id),
    user_id INTEGER NOT NULL REFERENCES users (id),
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_replies_thread ON replies (thread_id);

CREATE TABLE likes (
    user_id INTEGER NOT NULL REFERENCES users (id),
    kind TEXT NOT NULL CHECK (kind IN ('thread', 'reply')),
    target_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, kind, target_id)
);
CREATE INDEX idx_likes_target ON likes (kind, target_id);

-- +goose Down
DROP TABLE likes;
DROP TABLE replies;
DROP TABLE threads;
DROP TABLE sessions;
DROP TABLE users;
