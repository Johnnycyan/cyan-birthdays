package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GuildSettings represents per-guild configuration
type GuildSettings struct {
	GuildID            string
	ChannelID          *string
	RoleID             *string
	TimeUTC            int
	MessageWithYear    string
	MessageWithoutYear string
	AllowRoleMention   bool
	RequiredRoleID     *string
	DefaultTimezone    string
	SetupComplete      bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// MemberBirthday represents a member's birthday data
type MemberBirthday struct {
	GuildID   string
	UserID    string
	Month     int
	Day       int
	Year      *int
	Timezone  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Repository handles database operations
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new database repository
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetGuildSettings retrieves settings for a guild
func (r *Repository) GetGuildSettings(ctx context.Context, guildID string) (*GuildSettings, error) {
	var gs GuildSettings
	err := r.pool.QueryRow(ctx, `
		SELECT guild_id, channel_id, role_id, time_utc, message_with_year, 
		       message_without_year, allow_role_mention, required_role_id,
		       default_timezone, setup_complete, created_at, updated_at
		FROM guild_settings WHERE guild_id = $1
	`, guildID).Scan(
		&gs.GuildID, &gs.ChannelID, &gs.RoleID, &gs.TimeUTC,
		&gs.MessageWithYear, &gs.MessageWithoutYear, &gs.AllowRoleMention,
		&gs.RequiredRoleID, &gs.DefaultTimezone, &gs.SetupComplete,
		&gs.CreatedAt, &gs.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &gs, nil
}

// UpsertGuildSettings creates or updates guild settings
func (r *Repository) UpsertGuildSettings(ctx context.Context, gs *GuildSettings) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, channel_id, role_id, time_utc, 
		    message_with_year, message_without_year, allow_role_mention,
		    required_role_id, default_timezone, setup_complete, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    channel_id = EXCLUDED.channel_id,
		    role_id = EXCLUDED.role_id,
		    time_utc = EXCLUDED.time_utc,
		    message_with_year = EXCLUDED.message_with_year,
		    message_without_year = EXCLUDED.message_without_year,
		    allow_role_mention = EXCLUDED.allow_role_mention,
		    required_role_id = EXCLUDED.required_role_id,
		    default_timezone = EXCLUDED.default_timezone,
		    setup_complete = EXCLUDED.setup_complete,
		    updated_at = NOW()
	`, gs.GuildID, gs.ChannelID, gs.RoleID, gs.TimeUTC,
		gs.MessageWithYear, gs.MessageWithoutYear, gs.AllowRoleMention,
		gs.RequiredRoleID, gs.DefaultTimezone, gs.SetupComplete)
	return err
}

// UpdateGuildChannel updates only the channel_id
func (r *Repository) UpdateGuildChannel(ctx context.Context, guildID, channelID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, channel_id, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    channel_id = EXCLUDED.channel_id,
		    updated_at = NOW()
	`, guildID, channelID)
	return err
}

// UpdateGuildRole updates only the role_id
func (r *Repository) UpdateGuildRole(ctx context.Context, guildID, roleID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, role_id, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    role_id = EXCLUDED.role_id,
		    updated_at = NOW()
	`, guildID, roleID)
	return err
}

// UpdateGuildTime updates the announcement hour
func (r *Repository) UpdateGuildTime(ctx context.Context, guildID string, hour int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, time_utc, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    time_utc = EXCLUDED.time_utc,
		    updated_at = NOW()
	`, guildID, hour)
	return err
}

// UpdateGuildMessageWithYear updates the birthday message with year
func (r *Repository) UpdateGuildMessageWithYear(ctx context.Context, guildID, message string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, message_with_year, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    message_with_year = EXCLUDED.message_with_year,
		    updated_at = NOW()
	`, guildID, message)
	return err
}

// UpdateGuildMessageWithoutYear updates the birthday message without year
func (r *Repository) UpdateGuildMessageWithoutYear(ctx context.Context, guildID, message string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, message_without_year, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    message_without_year = EXCLUDED.message_without_year,
		    updated_at = NOW()
	`, guildID, message)
	return err
}

// UpdateGuildRoleMention updates the allow_role_mention setting
func (r *Repository) UpdateGuildRoleMention(ctx context.Context, guildID string, allow bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, allow_role_mention, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    allow_role_mention = EXCLUDED.allow_role_mention,
		    updated_at = NOW()
	`, guildID, allow)
	return err
}

// UpdateGuildRequiredRole updates the required_role_id
func (r *Repository) UpdateGuildRequiredRole(ctx context.Context, guildID string, roleID *string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, required_role_id, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    required_role_id = EXCLUDED.required_role_id,
		    updated_at = NOW()
	`, guildID, roleID)
	return err
}

// UpdateGuildDefaultTimezone updates the default timezone
func (r *Repository) UpdateGuildDefaultTimezone(ctx context.Context, guildID, timezone string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_settings (guild_id, default_timezone, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (guild_id) DO UPDATE SET
		    default_timezone = EXCLUDED.default_timezone,
		    updated_at = NOW()
	`, guildID, timezone)
	return err
}

// UpdateGuildSetupComplete marks setup as complete
func (r *Repository) UpdateGuildSetupComplete(ctx context.Context, guildID string, complete bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE guild_settings SET setup_complete = $2, updated_at = NOW()
		WHERE guild_id = $1
	`, guildID, complete)
	return err
}

// ClearGuildSettings deletes all settings for a guild
func (r *Repository) ClearGuildSettings(ctx context.Context, guildID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM guild_settings WHERE guild_id = $1`, guildID)
	return err
}

// GetAllSetupGuilds returns all guilds with completed setup
func (r *Repository) GetAllSetupGuilds(ctx context.Context) ([]GuildSettings, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT guild_id, channel_id, role_id, time_utc, message_with_year, 
		       message_without_year, allow_role_mention, required_role_id,
		       default_timezone, setup_complete, created_at, updated_at
		FROM guild_settings WHERE setup_complete = true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guilds []GuildSettings
	for rows.Next() {
		var gs GuildSettings
		if err := rows.Scan(
			&gs.GuildID, &gs.ChannelID, &gs.RoleID, &gs.TimeUTC,
			&gs.MessageWithYear, &gs.MessageWithoutYear, &gs.AllowRoleMention,
			&gs.RequiredRoleID, &gs.DefaultTimezone, &gs.SetupComplete,
			&gs.CreatedAt, &gs.UpdatedAt,
		); err != nil {
			return nil, err
		}
		guilds = append(guilds, gs)
	}
	return guilds, nil
}

// SetMemberBirthday creates or updates a member's birthday
func (r *Repository) SetMemberBirthday(ctx context.Context, mb *MemberBirthday) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO member_birthdays (guild_id, user_id, month, day, year, timezone, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (guild_id, user_id) DO UPDATE SET
		    month = EXCLUDED.month,
		    day = EXCLUDED.day,
		    year = EXCLUDED.year,
		    timezone = EXCLUDED.timezone,
		    updated_at = NOW()
	`, mb.GuildID, mb.UserID, mb.Month, mb.Day, mb.Year, mb.Timezone)
	return err
}

// GetMemberBirthday retrieves a member's birthday
func (r *Repository) GetMemberBirthday(ctx context.Context, guildID, userID string) (*MemberBirthday, error) {
	var mb MemberBirthday
	err := r.pool.QueryRow(ctx, `
		SELECT guild_id, user_id, month, day, year, timezone, created_at, updated_at
		FROM member_birthdays WHERE guild_id = $1 AND user_id = $2
	`, guildID, userID).Scan(
		&mb.GuildID, &mb.UserID, &mb.Month, &mb.Day, &mb.Year,
		&mb.Timezone, &mb.CreatedAt, &mb.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &mb, nil
}

// DeleteMemberBirthday removes a member's birthday
func (r *Repository) DeleteMemberBirthday(ctx context.Context, guildID, userID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM member_birthdays WHERE guild_id = $1 AND user_id = $2
	`, guildID, userID)
	return err
}

// GetBirthdaysForDate retrieves all birthdays for a specific month/day in a guild
func (r *Repository) GetBirthdaysForDate(ctx context.Context, guildID string, month, day int) ([]MemberBirthday, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT guild_id, user_id, month, day, year, timezone, created_at, updated_at
		FROM member_birthdays 
		WHERE guild_id = $1 AND month = $2 AND day = $3
	`, guildID, month, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var birthdays []MemberBirthday
	for rows.Next() {
		var mb MemberBirthday
		if err := rows.Scan(
			&mb.GuildID, &mb.UserID, &mb.Month, &mb.Day, &mb.Year,
			&mb.Timezone, &mb.CreatedAt, &mb.UpdatedAt,
		); err != nil {
			return nil, err
		}
		birthdays = append(birthdays, mb)
	}
	return birthdays, nil
}

// GetUpcomingBirthdays retrieves birthdays within the next N days for a guild
func (r *Repository) GetUpcomingBirthdays(ctx context.Context, guildID string, days int) ([]MemberBirthday, error) {
	// We need to check for wrap-around at year end
	rows, err := r.pool.Query(ctx, `
		SELECT guild_id, user_id, month, day, year, timezone, created_at, updated_at
		FROM member_birthdays 
		WHERE guild_id = $1
		ORDER BY month, day
	`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var birthdays []MemberBirthday
	for rows.Next() {
		var mb MemberBirthday
		if err := rows.Scan(
			&mb.GuildID, &mb.UserID, &mb.Month, &mb.Day, &mb.Year,
			&mb.Timezone, &mb.CreatedAt, &mb.UpdatedAt,
		); err != nil {
			return nil, err
		}
		birthdays = append(birthdays, mb)
	}
	return birthdays, nil
}

// GetAllGuildBirthdays retrieves all birthdays for a guild
func (r *Repository) GetAllGuildBirthdays(ctx context.Context, guildID string) ([]MemberBirthday, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT guild_id, user_id, month, day, year, timezone, created_at, updated_at
		FROM member_birthdays WHERE guild_id = $1
		ORDER BY month, day
	`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var birthdays []MemberBirthday
	for rows.Next() {
		var mb MemberBirthday
		if err := rows.Scan(
			&mb.GuildID, &mb.UserID, &mb.Month, &mb.Day, &mb.Year,
			&mb.Timezone, &mb.CreatedAt, &mb.UpdatedAt,
		); err != nil {
			return nil, err
		}
		birthdays = append(birthdays, mb)
	}
	return birthdays, nil
}
