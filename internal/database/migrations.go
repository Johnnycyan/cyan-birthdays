package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS guild_settings (
    guild_id           VARCHAR(32) PRIMARY KEY,
    channel_id         VARCHAR(32),
    role_id            VARCHAR(32),
    time_utc           INTEGER DEFAULT 0,
    message_with_year  TEXT DEFAULT '{mention} has turned {new_age}, happy birthday!',
    message_without_year TEXT DEFAULT 'Happy birthday {mention}!',
    allow_role_mention BOOLEAN DEFAULT FALSE,
    required_role_id   VARCHAR(32),
    default_timezone   VARCHAR(64) DEFAULT 'UTC',
    setup_complete     BOOLEAN DEFAULT FALSE,
    created_at         TIMESTAMP DEFAULT NOW(),
    updated_at         TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS member_birthdays (
    guild_id   VARCHAR(32) NOT NULL,
    user_id    VARCHAR(32) NOT NULL,
    month      INTEGER NOT NULL,
    day        INTEGER NOT NULL,
    year       INTEGER,
    timezone   VARCHAR(64) DEFAULT 'UTC',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_birthdays_date ON member_birthdays(month, day);
`

// Migrate runs the database migrations
func Migrate(pool *pgxpool.Pool) error {
	ctx := context.Background()
	_, err := pool.Exec(ctx, schema)
	return err
}
