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
    european_date_format BOOLEAN DEFAULT FALSE,
    use_24h_time       BOOLEAN DEFAULT FALSE,
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

// migrations to add new columns to existing tables
const migrations = `
-- Add european_date_format column if it doesn't exist
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='guild_settings' AND column_name='european_date_format') THEN
        ALTER TABLE guild_settings ADD COLUMN european_date_format BOOLEAN DEFAULT FALSE;
    END IF;
END $$;

-- Add use_24h_time column if it doesn't exist
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='guild_settings' AND column_name='use_24h_time') THEN
        ALTER TABLE guild_settings ADD COLUMN use_24h_time BOOLEAN DEFAULT FALSE;
    END IF;
END $$;
`

// Migrate runs the database migrations
func Migrate(pool *pgxpool.Pool) error {
	ctx := context.Background()
	
	// Create tables
	if _, err := pool.Exec(ctx, schema); err != nil {
		return err
	}
	
	// Run migrations for new columns
	if _, err := pool.Exec(ctx, migrations); err != nil {
		return err
	}
	
	return nil
}
