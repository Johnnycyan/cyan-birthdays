package bot

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "birthday",
		Description: "Set and manage your birthday",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "set",
				Description: "Set your birthday",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "birthday",
						Description: "Your birthday (e.g., 9/24 or September 24, 2002)",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
					{
						Name:         "timezone",
						Description:  "Your timezone",
						Type:         discordgo.ApplicationCommandOptionString,
						Required:     false,
						Autocomplete: true,
					},
				},
			},
			{
				Name:        "remove",
				Description: "Remove your birthday",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "upcoming",
				Description: "View upcoming birthdays",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "days",
						Description: "Number of days to look ahead (default: 7)",
						Type:        discordgo.ApplicationCommandOptionInteger,
						MinValue:    floatPtr(1),
						MaxValue:    365,
						Required:    false,
					},
				},
			},
		},
	},
	{
		Name:        "bdset",
		Description: "Birthday settings for admins",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "channel",
				Description: "Set the birthday announcement channel",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "channel",
						Description: "The channel for birthday announcements",
						Type:        discordgo.ApplicationCommandOptionChannel,
						ChannelTypes: []discordgo.ChannelType{
							discordgo.ChannelTypeGuildText,
						},
						Required: true,
					},
				},
			},
			{
				Name:        "role",
				Description: "Set the birthday role",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "role",
						Description: "The role to give on birthdays",
						Type:        discordgo.ApplicationCommandOptionRole,
						Required:    true,
					},
				},
			},
			{
				Name:        "time",
				Description: "Set the announcement hour (0-23 in server's default timezone)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "hour",
						Description: "Hour of day (0-23)",
						Type:        discordgo.ApplicationCommandOptionInteger,
						MinValue:    floatPtr(0),
						MaxValue:    23,
						Required:    true,
					},
				},
			},
			{
				Name:        "msgwithyear",
				Description: "Set the birthday message (with age)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "message",
						Description: "Message with placeholders: {mention}, {name}, {new_age}",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
				},
			},
			{
				Name:        "msgwithoutyear",
				Description: "Set the birthday message (without age)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "message",
						Description: "Message with placeholders: {mention}, {name}",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
				},
			},
			{
				Name:        "rolemention",
				Description: "Toggle allowing role mentions in birthday messages",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "enabled",
						Description: "Allow role mentions?",
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Required:    true,
					},
				},
			},
			{
				Name:        "requiredrole",
				Description: "Set a role required for birthday announcements",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "role",
						Description: "The required role (leave empty to remove)",
						Type:        discordgo.ApplicationCommandOptionRole,
						Required:    false,
					},
				},
			},
			{
				Name:        "defaulttimezone",
				Description: "Set the default timezone for users",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:         "timezone",
						Description:  "Search for a timezone",
						Type:         discordgo.ApplicationCommandOptionString,
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Name:        "force",
				Description: "Force-set a user's birthday",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "user",
						Description: "The user to set birthday for",
						Type:        discordgo.ApplicationCommandOptionUser,
						Required:    true,
					},
				},
			},
			{
				Name:        "settings",
				Description: "View current birthday settings",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "stop",
				Description: "Clear all birthday settings for this server",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "interactive",
				Description: "Start interactive setup wizard",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "dateformat",
				Description: "Toggle European date format (DD/MM instead of MM/DD)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "european",
						Description: "Use DD/MM/YYYY format?",
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Required:    true,
					},
				},
			},
			{
				Name:        "timeformat",
				Description: "Toggle 24-hour time display",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "use24h",
						Description: "Use 24-hour time format?",
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Required:    true,
					},
				},
			},
			{
				Name:        "import",
				Description: "Import birthdays from RedBot Birthday cog JSON file (bot owner only)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "file",
						Description: "The JSON file from the RedBot cog",
						Type:        discordgo.ApplicationCommandOptionAttachment,
						Required:    true,
					},
				},
			},
			{
				Name:        "admin",
				Description: "Manage bot admins",
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "add",
						Description: "Add a user or role as bot admin",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Options: []*discordgo.ApplicationCommandOption{
							{
								Name:        "user",
								Description: "User to add as admin",
								Type:        discordgo.ApplicationCommandOptionUser,
								Required:    false,
							},
							{
								Name:        "role",
								Description: "Role to add as admin",
								Type:        discordgo.ApplicationCommandOptionRole,
								Required:    false,
							},
						},
					},
					{
						Name:        "remove",
						Description: "Remove a user or role from bot admins",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Options: []*discordgo.ApplicationCommandOption{
							{
								Name:        "user",
								Description: "User to remove from admins",
								Type:        discordgo.ApplicationCommandOptionUser,
								Required:    false,
							},
							{
								Name:        "role",
								Description: "Role to remove from admins",
								Type:        discordgo.ApplicationCommandOptionRole,
								Required:    false,
							},
						},
					},
					{
						Name:        "list",
						Description: "List all bot admins",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
		},
	},
}

func floatPtr(f float64) *float64 {
	return &f
}

func int64Ptr(i int64) *int64 {
	return &i
}

// registerCommands registers all slash commands globally
func (b *Bot) registerCommands() error {
	slog.Info("Registering slash commands...")

	_, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, "", commands)
	if err != nil {
		return err
	}

	slog.Info("Successfully registered slash commands", "count", len(commands))
	return nil
}

// unregisterCommands removes all slash commands
func (b *Bot) unregisterCommands() error {
	registeredCmds, err := b.session.ApplicationCommands(b.session.State.User.ID, "")
	if err != nil {
		return err
	}

	for _, cmd := range registeredCmds {
		if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, "", cmd.ID); err != nil {
			slog.Warn("Failed to delete command", "name", cmd.Name, "error", err)
		}
	}

	return nil
}
