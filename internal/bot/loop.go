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

	// Then run every hour at the top of the hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			slog.Info("Birthday loop stopped")
			return
		case <-ticker.C:
			b.processBirthdays()
		}
	}
}

// processBirthdays checks all guilds for birthdays to announce
func (b *Bot) processBirthdays() {
	ctx := context.Background()

	// Get all guilds with setup complete
	guilds, err := b.repo.GetAllSetupGuilds(ctx)
	if err != nil {
		slog.Error("Failed to get guilds for birthday processing", "error", err)
		return
	}

	slog.Debug("Processing birthdays", "guild_count", len(guilds))

	for _, gs := range guilds {
		b.processGuildBirthdays(ctx, gs)
	}
}

// processGuildBirthdays processes birthdays for a single guild
func (b *Bot) processGuildBirthdays(ctx context.Context, gs database.GuildSettings) {
	if gs.ChannelID == nil || gs.RoleID == nil {
		return
	}

	// Get all birthdays for this guild
	birthdays, err := b.repo.GetAllGuildBirthdays(ctx, gs.GuildID)
	if err != nil {
		slog.Error("Failed to get birthdays for guild", "guild_id", gs.GuildID, "error", err)
		return
	}

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

	// Remove birthday role from members who no longer have their birthday
	b.cleanupBirthdayRoles(ctx, gs, birthdays, guild)
}

// processMemberBirthday checks if a member should be announced
func (b *Bot) processMemberBirthday(ctx context.Context, gs database.GuildSettings, bd database.MemberBirthday, guild *discordgo.Guild) {
	// Check if it's their birthday in their timezone
	isBirthday, err := timezone.IsBirthdayToday(bd.Month, bd.Day, bd.Timezone)
	if err != nil {
		slog.Warn("Failed to check birthday timezone", "user_id", bd.UserID, "timezone", bd.Timezone, "error", err)
		// Fall back to UTC
		isBirthday, _ = timezone.IsBirthdayToday(bd.Month, bd.Day, "UTC")
	}

	if !isBirthday {
		return
	}

	// Check if current hour in user's timezone matches the announcement hour
	shouldAnnounce, err := timezone.ShouldAnnounce(gs.TimeUTC, bd.Timezone)
	if err != nil {
		slog.Warn("Failed to check announcement time", "user_id", bd.UserID, "error", err)
		return
	}

	if !shouldAnnounce {
		return
	}

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
		return
	}

	// Add birthday role
	if err := b.session.GuildMemberRoleAdd(gs.GuildID, bd.UserID, *gs.RoleID); err != nil {
		slog.Error("Failed to add birthday role", "guild_id", gs.GuildID, "user_id", bd.UserID, "error", err)
	} else {
		slog.Info("Added birthday role", "guild_id", gs.GuildID, "user_id", bd.UserID)
	}

	// Send announcement
	var message string
	if bd.Year != nil && *bd.Year > 0 {
		// Calculate age
		now := time.Now()
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

// cleanupBirthdayRoles removes birthday role from members whose birthday has ended
func (b *Bot) cleanupBirthdayRoles(ctx context.Context, gs database.GuildSettings, birthdays []database.MemberBirthday, guild *discordgo.Guild) {
	if gs.RoleID == nil {
		return
	}

	// Build set of users who should have the role today
	birthdayUsers := make(map[string]bool)
	for _, bd := range birthdays {
		isBirthday, _ := timezone.IsBirthdayToday(bd.Month, bd.Day, bd.Timezone)
		if isBirthday {
			birthdayUsers[bd.UserID] = true
		}
	}

	// Check all members with the birthday role
	role, err := b.session.State.Role(gs.GuildID, *gs.RoleID)
	if err != nil {
		return
	}

	// Get guild members (this may need pagination for large guilds)
	members, err := b.session.GuildMembers(gs.GuildID, "", 1000)
	if err != nil {
		slog.Warn("Failed to get guild members for cleanup", "guild_id", gs.GuildID, "error", err)
		return
	}

	for _, member := range members {
		hasRole := false
		for _, roleID := range member.Roles {
			if roleID == role.ID {
				hasRole = true
				break
			}
		}

		if hasRole && !birthdayUsers[member.User.ID] {
			// Remove role - birthday is over
			if err := b.session.GuildMemberRoleRemove(gs.GuildID, member.User.ID, *gs.RoleID); err != nil {
				slog.Warn("Failed to remove birthday role", "guild_id", gs.GuildID, "user_id", member.User.ID, "error", err)
			} else {
				slog.Info("Removed birthday role", "guild_id", gs.GuildID, "user_id", member.User.ID)
			}
		}
	}
}
