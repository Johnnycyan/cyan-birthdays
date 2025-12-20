package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/Johnnycyan/cyan-birthdays/internal/database"
	"github.com/Johnnycyan/cyan-birthdays/internal/timezone"
	"github.com/bwmarrin/discordgo"
)

// startBirthdayLoop runs the hourly birthday check
func (b *Bot) startBirthdayLoop() {
	slog.Info("Starting birthday loop")

	// Run immediately on start
	b.processBirthdays()

	// Calculate time until next hour
	now := time.Now()
	nextHour := now.Truncate(time.Hour).Add(time.Hour)
	timeToNextHour := time.Until(nextHour)
	
	slog.Info("Scheduling next birthday check", "next_check", nextHour.Format("15:04:05"), "wait_duration", timeToNextHour.String())

	// Wait until the top of the next hour
	select {
	case <-b.stopCh:
		slog.Info("Birthday loop stopped before first scheduled run")
		return
	case <-time.After(timeToNextHour):
		b.processBirthdays()
	}

	// Then run every hour at the top of the hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			slog.Info("Birthday loop stopped")
			return
		case t := <-ticker.C:
			slog.Debug("Birthday ticker fired", "time", t.Format("15:04:05"))
			b.processBirthdays()
		}
	}
}

// processBirthdays checks all guilds for birthdays to announce
func (b *Bot) processBirthdays() {
	ctx := context.Background()
	now := time.Now()
	
	slog.Info("Processing birthdays", "current_time_utc", now.UTC().Format("2006-01-02 15:04:05"))

	// First, cleanup any expired birthday roles across all guilds
	b.cleanupExpiredBirthdayRoles(ctx)

	// Get all guilds with setup complete
	guilds, err := b.repo.GetAllSetupGuilds(ctx)
	if err != nil {
		slog.Error("Failed to get guilds for birthday processing", "error", err)
		return
	}

	slog.Debug("Processing guilds", "guild_count", len(guilds))

	for _, gs := range guilds {
		b.processGuildBirthdays(ctx, gs)
	}
	
	slog.Debug("Birthday processing complete")
}

// processGuildBirthdays processes birthdays for a single guild
func (b *Bot) processGuildBirthdays(ctx context.Context, gs database.GuildSettings) {
	if gs.ChannelID == nil || gs.RoleID == nil {
		slog.Debug("Guild missing channel or role", "guild_id", gs.GuildID, "has_channel", gs.ChannelID != nil, "has_role", gs.RoleID != nil)
		return
	}

	slog.Debug("Processing guild birthdays", "guild_id", gs.GuildID, "announcement_hour", gs.TimeUTC, "default_tz", gs.DefaultTimezone)

	// Get all birthdays for this guild
	birthdays, err := b.repo.GetAllGuildBirthdays(ctx, gs.GuildID)
	if err != nil {
		slog.Error("Failed to get birthdays for guild", "guild_id", gs.GuildID, "error", err)
		return
	}

	slog.Debug("Found birthdays in guild", "guild_id", gs.GuildID, "count", len(birthdays))

	guild, err := b.session.State.Guild(gs.GuildID)
	if err != nil {
		// Try to fetch if not in state
		guild, err = b.session.Guild(gs.GuildID)
		if err != nil {
			slog.Debug("Guild not accessible", "guild_id", gs.GuildID)
			return
		}
	}

	for _, bd := range birthdays {
		b.processMemberBirthday(ctx, gs, bd, guild)
	}
}

// processMemberBirthday checks if a member should be announced
func (b *Bot) processMemberBirthday(ctx context.Context, gs database.GuildSettings, bd database.MemberBirthday, guild *discordgo.Guild) {
	slog.Debug("Checking member birthday", "guild_id", gs.GuildID, "user_id", bd.UserID, "month", bd.Month, "day", bd.Day, "timezone", bd.Timezone)
	
	// Check if it's their birthday in their timezone
	isBirthday, err := timezone.IsBirthdayToday(bd.Month, bd.Day, bd.Timezone)
	if err != nil {
		slog.Warn("Failed to check birthday timezone", "user_id", bd.UserID, "timezone", bd.Timezone, "error", err)
		// Fall back to UTC
		isBirthday, _ = timezone.IsBirthdayToday(bd.Month, bd.Day, "UTC")
	}

	slog.Debug("Birthday check result", "user_id", bd.UserID, "is_birthday", isBirthday)

	if !isBirthday {
		return
	}

	// Check if current hour in user's timezone matches the announcement hour
	shouldAnnounce, err := timezone.ShouldAnnounce(gs.TimeUTC, bd.Timezone)
	if err != nil {
		slog.Warn("Failed to check announcement time", "user_id", bd.UserID, "error", err)
		return
	}

	slog.Debug("Announcement check", "user_id", bd.UserID, "should_announce", shouldAnnounce, "configured_hour", gs.TimeUTC, "user_tz", bd.Timezone)

	if !shouldAnnounce {
		return
	}

	slog.Info("Processing birthday announcement", "guild_id", gs.GuildID, "user_id", bd.UserID)

	// Get the member
	member, err := b.session.GuildMember(gs.GuildID, bd.UserID)
	if err != nil {
		slog.Debug("Member not found", "guild_id", gs.GuildID, "user_id", bd.UserID)
		return
	}

	// Check required role
	if gs.RequiredRoleID != nil {
		hasRole := false
		for _, roleID := range member.Roles {
			if roleID == *gs.RequiredRoleID {
				hasRole = true
				break
			}
		}
		if !hasRole {
			slog.Debug("Member missing required role", "user_id", bd.UserID, "required_role", *gs.RequiredRoleID)
			return
		}
	}

	// Check if already has birthday role
	hasRole := false
	for _, roleID := range member.Roles {
		if roleID == *gs.RoleID {
			hasRole = true
			break
		}
	}

	if hasRole {
		// Already announced today
		slog.Debug("Member already has birthday role, skipping", "user_id", bd.UserID)
		return
	}

	// Add birthday role
	if err := b.session.GuildMemberRoleAdd(gs.GuildID, bd.UserID, *gs.RoleID); err != nil {
		slog.Error("Failed to add birthday role", "guild_id", gs.GuildID, "user_id", bd.UserID, "error", err)
		return
	}
	slog.Info("Added birthday role", "guild_id", gs.GuildID, "user_id", bd.UserID)

	// Calculate expiration time: announcement hour + 24h in user's timezone
	loc, err := time.LoadLocation(bd.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	// Get today's announcement time in user's timezone
	announcementTime := time.Date(now.Year(), now.Month(), now.Day(), gs.TimeUTC, 0, 0, 0, loc)
	// If we're past the announcement hour (bot started late), the base is still today's announcement time
	// Add 24 hours for expiration
	expiresAt := announcementTime.Add(24 * time.Hour).UTC()
	
	slog.Debug("Setting birthday role expiration", "user_id", bd.UserID, "expires_at", expiresAt)
	if err := b.repo.SetActiveBirthdayRole(ctx, gs.GuildID, bd.UserID, expiresAt); err != nil {
		slog.Error("Failed to record birthday role expiration", "error", err)
	}

	// Send announcement
	var message string
	if bd.Year != nil && *bd.Year > 0 {
		// Calculate age
		age := now.Year() - *bd.Year
		message = formatMessage(gs.MessageWithYear, member.User.Username, bd.UserID, &age)
	} else {
		message = formatMessage(gs.MessageWithoutYear, member.User.Username, bd.UserID, nil)
	}

	allowedMentions := &discordgo.MessageAllowedMentions{
		Users: []string{bd.UserID},
	}
	if gs.AllowRoleMention {
		allowedMentions.Parse = []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles}
	}

	_, err = b.session.ChannelMessageSendComplex(*gs.ChannelID, &discordgo.MessageSend{
		Content:         message,
		AllowedMentions: allowedMentions,
	})
	if err != nil {
		slog.Error("Failed to send birthday message", "guild_id", gs.GuildID, "channel_id", *gs.ChannelID, "error", err)
	} else {
		slog.Info("Sent birthday announcement", "guild_id", gs.GuildID, "user_id", bd.UserID)
	}
}

// cleanupExpiredBirthdayRoles removes birthday roles that have exceeded their 24h period
func (b *Bot) cleanupExpiredBirthdayRoles(ctx context.Context) {
	slog.Debug("Checking for expired birthday roles")
	
	expiredRoles, err := b.repo.GetExpiredBirthdayRoles(ctx)
	if err != nil {
		slog.Error("Failed to get expired birthday roles", "error", err)
		return
	}

	if len(expiredRoles) == 0 {
		slog.Debug("No expired birthday roles found")
		return
	}

	slog.Info("Found expired birthday roles to remove", "count", len(expiredRoles))

	for _, ar := range expiredRoles {
		// Get guild settings to find the role ID
		gs, err := b.repo.GetGuildSettings(ctx, ar.GuildID)
		if err != nil {
			slog.Warn("Failed to get guild settings for cleanup", "guild_id", ar.GuildID, "error", err)
			// Still delete the record
			b.repo.DeleteActiveBirthdayRole(ctx, ar.GuildID, ar.UserID)
			continue
		}

		if gs.RoleID == nil {
			b.repo.DeleteActiveBirthdayRole(ctx, ar.GuildID, ar.UserID)
			continue
		}

		// Remove the role from the member
		if err := b.session.GuildMemberRoleRemove(ar.GuildID, ar.UserID, *gs.RoleID); err != nil {
			slog.Warn("Failed to remove expired birthday role", "guild_id", ar.GuildID, "user_id", ar.UserID, "error", err)
		} else {
			slog.Info("Removed expired birthday role", "guild_id", ar.GuildID, "user_id", ar.UserID, "expired_at", ar.RoleExpiresAt)
		}

		// Delete the record regardless of role removal success
		if err := b.repo.DeleteActiveBirthdayRole(ctx, ar.GuildID, ar.UserID); err != nil {
			slog.Error("Failed to delete active birthday role record", "error", err)
		}
	}
}
