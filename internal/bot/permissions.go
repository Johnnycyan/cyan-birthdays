package bot

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

// HasBotAdminPermission checks if a user has permission to use admin commands
// Returns true if:
// 1. User is the bot owner (from OWNER_ID env var)
// 2. User has Discord's "Manage Server" permission
// 3. User or one of their roles is in the bot_admins table for this guild
func (b *Bot) HasBotAdminPermission(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	// 1. Check if user is bot owner
	if b.config.OwnerID != "" && b.config.OwnerID == i.Member.User.ID {
		return true
	}

	// 2. Check Discord's "Manage Server" permission
	if i.Member.Permissions&discordgo.PermissionManageServer != 0 {
		return true
	}

	// 3. Check database for bot admin entries
	ctx := context.Background()
	isAdmin, err := b.repo.IsBotAdmin(ctx, i.GuildID, i.Member.User.ID, i.Member.Roles)
	if err != nil {
		// On error, fall back to denying permission
		return false
	}
	return isAdmin
}
