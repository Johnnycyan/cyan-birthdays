package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Johnnycyan/cyan-birthdays/internal/database"
	"github.com/Johnnycyan/cyan-birthdays/internal/timezone"
	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
)

// handleCommand routes commands to their handlers
func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	
	switch data.Name {
	case "birthday":
		b.handleBirthdayCommand(s, i)
	case "bdset":
		b.handleBdsetCommand(s, i)
	}
}

// handleBirthdayCommand handles /birthday subcommands
func (b *Bot) handleBirthdayCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if len(i.ApplicationCommandData().Options) == 0 {
		return
	}

	subcommand := i.ApplicationCommandData().Options[0].Name

	switch subcommand {
	case "set":
		b.handleBirthdaySet(s, i)
	case "remove":
		b.handleBirthdayRemove(s, i)
	case "upcoming":
		b.handleBirthdayUpcoming(s, i)
	}
}

// handleBirthdaySet sets a user's birthday from command options
func (b *Bot) handleBirthdaySet(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	
	slog.Debug("Birthday set called", "guild", i.GuildID, "user", i.Member.User.ID, "opts_count", len(opts))
	
	var dateStr, tzStr string
	for _, opt := range opts {
		switch opt.Name {
		case "birthday":
			dateStr = opt.StringValue()
		case "timezone":
			tzStr = opt.StringValue()
		}
	}

	slog.Debug("Parsed options", "dateStr", dateStr, "tzStr", tzStr)

	ctx := context.Background()
	formatSettings := b.GetFormatSettings(ctx, i.GuildID)

	// Get default timezone if not provided
	if tzStr == "" {
		gs, err := b.repo.GetGuildSettings(ctx, i.GuildID)
		if err == nil && gs != nil {
			tzStr = gs.DefaultTimezone
		} else {
			tzStr = "UTC"
		}
	}

	// Parse the date with format settings
	month, day, year, err := ParseDateWithSettings(dateStr, formatSettings)
	if err != nil {
		slog.Debug("Failed to parse date", "dateStr", dateStr, "error", err)
		formatHint := "MM/DD"
		if formatSettings.EuropeanDateFormat {
			formatHint = "DD/MM"
		}
		respondError(s, i, fmt.Sprintf("Invalid date format. Use formats like: %s, September 24, or %s/2002", formatHint, formatHint))
		return
	}

	slog.Debug("Parsed date", "month", month, "day", day, "year", year)

	// Validate timezone
	if !timezone.ValidateTimezone(tzStr) {
		respondError(s, i, fmt.Sprintf("Invalid timezone: %s. Please select from the autocomplete list.", tzStr))
		return
	}

	// Save to database
	mb := &database.MemberBirthday{
		GuildID:  i.GuildID,
		UserID:   i.Member.User.ID,
		Month:    month,
		Day:      day,
		Year:     year,
		Timezone: tzStr,
	}
	
	slog.Debug("Saving birthday", "guildID", mb.GuildID, "userID", mb.UserID, "month", mb.Month, "day", mb.Day)
	
	if err := b.repo.SetMemberBirthday(ctx, mb); err != nil {
		slog.Error("Failed to save birthday", "error", err)
		respondError(s, i, "Failed to save your birthday")
		return
	}

	slog.Info("Birthday saved successfully", "guildID", mb.GuildID, "userID", mb.UserID)

	// Format confirmation using guild settings
	dateDisplay := FormatDate(month, day, year, formatSettings)
	currentTime, _ := timezone.GetCurrentTime(tzStr)
	timeDisplay := FormatTime(currentTime, formatSettings)
	
	respondEphemeral(s, i, fmt.Sprintf(
		"🎂 Your birthday has been set to **%s**!\nTimezone: %s (current time: %s)",
		dateDisplay, tzStr, timeDisplay,
	))
}

// handleBirthdayRemove shows confirmation for removing birthday
func (b *Bot) handleBirthdayRemove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Are you sure you want to remove your birthday?",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Yes, remove it",
							Style:    discordgo.DangerButton,
							CustomID: "birthday_remove_confirm",
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.SecondaryButton,
							CustomID: "birthday_remove_cancel",
						},
					},
				},
			},
		},
	})
}

// handleBirthdayUpcoming lists upcoming birthdays
func (b *Bot) handleBirthdayUpcoming(s *discordgo.Session, i *discordgo.InteractionCreate) {
	days := 7
	opts := i.ApplicationCommandData().Options[0].Options
	for _, opt := range opts {
		if opt.Name == "days" {
			days = int(opt.IntValue())
		}
	}

	ctx := context.Background()
	birthdays, err := b.repo.GetAllGuildBirthdays(ctx, i.GuildID)
	if err != nil {
		respondError(s, i, "Failed to fetch birthdays")
		return
	}

	if len(birthdays) == 0 {
		respondEphemeral(s, i, "No birthdays have been set in this server yet.")
		return
	}

	// Get guild settings for announcement hour
	gs, err := b.repo.GetGuildSettings(ctx, i.GuildID)
	var announcementHour int
	if err != nil || gs == nil {
		announcementHour = 0 // Default to midnight
	} else {
		announcementHour = gs.TimeUTC
	}

	// Filter to upcoming birthdays
	now := time.Now().UTC()
	// Truncate to start of day for accurate date comparison
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	
	slog.Debug("Checking upcoming birthdays", "today", today.Format("2006-01-02"), "totalBirthdays", len(birthdays))
	
	type upcomingBday struct {
		UserID   string
		Month    int
		Day      int
		Year     *int
		Timezone string
		DaysAway int
	}

	var upcoming []upcomingBday
	for _, bd := range birthdays {
		// Calculate days until birthday using date-only comparison
		thisYearBday := time.Date(now.Year(), time.Month(bd.Month), bd.Day, 0, 0, 0, 0, time.UTC)
		
		// If birthday this year is before today, use next year
		if thisYearBday.Before(today) {
			thisYearBday = thisYearBday.AddDate(1, 0, 0)
		}
		
		daysAway := int(thisYearBday.Sub(today).Hours() / 24)
		
		slog.Debug("Checking birthday", "userID", bd.UserID, "month", bd.Month, "day", bd.Day, "thisYearBday", thisYearBday.Format("2006-01-02"), "daysAway", daysAway)
		
		if daysAway <= days {
			// Check if required role is set and user has it
			if gs.RequiredRoleID != nil {
				member, err := s.GuildMember(i.GuildID, bd.UserID)
				if err != nil {
					slog.Debug("Could not fetch member for role check", "userID", bd.UserID, "error", err)
					continue // Skip user if we can't check their roles
				}
				
				hasRole := false
				for _, roleID := range member.Roles {
					if roleID == *gs.RequiredRoleID {
						hasRole = true
						break
					}
				}
				if !hasRole {
					slog.Debug("User missing required role, skipping from upcoming", "userID", bd.UserID)
					continue
				}
			}
			
			upcoming = append(upcoming, upcomingBday{
				UserID:   bd.UserID,
				Month:    bd.Month,
				Day:      bd.Day,
				Year:     bd.Year,
				Timezone: bd.Timezone,
				DaysAway: daysAway,
			})
		}
	}

	slog.Debug("Upcoming birthdays filtered", "count", len(upcoming))

	if len(upcoming) == 0 {
		respondEphemeral(s, i, fmt.Sprintf("No upcoming birthdays in the next %d days.", days))
		return
	}

	// Sort by days away
	for i := 0; i < len(upcoming)-1; i++ {
		for j := i + 1; j < len(upcoming); j++ {
			if upcoming[j].DaysAway < upcoming[i].DaysAway {
				upcoming[i], upcoming[j] = upcoming[j], upcoming[i]
			}
		}
	}

	// Build embed
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("🎂 Upcoming Birthdays (Next %d Days)", days),
		Color: 0x00D9FF, // Cyan
	}

	// Group by date
	dateMap := make(map[string][]string)
	for _, bd := range upcoming {
		dateKey := fmt.Sprintf("%s %d", time.Month(bd.Month).String(), bd.Day)
		switch bd.DaysAway {
		case 0:
			dateKey = "Today!"
		case 1:
			dateKey = "Tomorrow"
		default:
			dateKey = fmt.Sprintf("In %d days", bd.DaysAway)
		}
		
		// Calculate announcement time in user's timezone, then convert to Unix timestamp
		loc, err := time.LoadLocation(bd.Timezone)
		if err != nil {
			loc = time.UTC
		}
		
		// Get the birthday date in user's timezone with announcement hour
		bdayDate := time.Date(now.Year(), time.Month(bd.Month), bd.Day, announcementHour, 0, 0, 0, loc)
		if bdayDate.Before(now) && bd.DaysAway > 0 {
			bdayDate = bdayDate.AddDate(1, 0, 0)
		}
		
		// Format as Discord timestamp (shows time only in viewer's local time)
		timestamp := fmt.Sprintf("<t:%d:t>", bdayDate.Unix())
		
		mention := fmt.Sprintf("<@%s> - %s", bd.UserID, timestamp)
		if bd.Year != nil && *bd.Year > 0 {
			age := now.Year() - *bd.Year
			if bd.DaysAway > 0 {
				mention = fmt.Sprintf("<@%s> (turning %d) - %s", bd.UserID, age, timestamp)
			} else {
				mention = fmt.Sprintf("<@%s> (now %d) - %s", bd.UserID, age, timestamp)
			}
		}
		dateMap[dateKey] = append(dateMap[dateKey], mention)
	}

	for date, users := range dateMap {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   date,
			Value:  strings.Join(users, "\n"),
			Inline: false,
		})
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}); err != nil {
		slog.Error("Failed to respond with upcoming birthdays embed", "error", err)
	}
}

// handleBdsetCommand handles /bdset subcommands
func (b *Bot) handleBdsetCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if len(i.ApplicationCommandData().Options) == 0 {
		return
	}

	// Check if user has permission to use admin commands
	if !b.HasBotAdminPermission(s, i) {
		respondError(s, i, "You don't have permission to use this command. You need Manage Server permission or be a bot admin.")
		return
	}

	subcommand := i.ApplicationCommandData().Options[0].Name

	switch subcommand {
	case "channel":
		b.handleBdsetChannel(s, i)
	case "role":
		b.handleBdsetRole(s, i)
	case "time":
		b.handleBdsetTime(s, i)
	case "msgwithyear":
		b.handleBdsetMsgWithYear(s, i)
	case "msgwithoutyear":
		b.handleBdsetMsgWithoutYear(s, i)
	case "rolemention":
		b.handleBdsetRoleMention(s, i)
	case "requiredrole":
		b.handleBdsetRequiredRole(s, i)
	case "defaulttimezone":
		b.handleBdsetDefaultTimezone(s, i)
	case "force":
		b.handleBdsetForce(s, i)
	case "settings":
		b.handleBdsetSettings(s, i)
	case "stop":
		b.handleBdsetStop(s, i)
	case "interactive":
		b.handleBdsetInteractive(s, i)
	case "dateformat":
		b.handleBdsetDateFormat(s, i)
	case "timeformat":
		b.handleBdsetTimeFormat(s, i)
	case "import":
		b.handleBdsetImport(s, i)
	case "admin":
		b.handleBdsetAdmin(s, i)
	}
}

// handleBdsetChannel sets the birthday announcement channel
func (b *Bot) handleBdsetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	channelID := opts[0].ChannelValue(s).ID

	ctx := context.Background()
	if err := b.repo.UpdateGuildChannel(ctx, i.GuildID, channelID); err != nil {
		respondError(s, i, "Failed to update channel")
		return
	}

	b.checkSetupComplete(ctx, i.GuildID)
	respondEphemeral(s, i, fmt.Sprintf("✅ Birthday channel set to <#%s>", channelID))
}

// handleBdsetRole sets the birthday role
func (b *Bot) handleBdsetRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	roleID := opts[0].RoleValue(s, i.GuildID).ID

	ctx := context.Background()
	if err := b.repo.UpdateGuildRole(ctx, i.GuildID, roleID); err != nil {
		respondError(s, i, "Failed to update role")
		return
	}

	b.checkSetupComplete(ctx, i.GuildID)
	respondEphemeral(s, i, fmt.Sprintf("✅ Birthday role set to <@&%s>", roleID))
}

// handleBdsetTime sets the announcement hour
func (b *Bot) handleBdsetTime(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	hour := int(opts[0].IntValue())

	ctx := context.Background()
	if err := b.repo.UpdateGuildTime(ctx, i.GuildID, hour); err != nil {
		respondError(s, i, "Failed to update time")
		return
	}

	b.checkSetupComplete(ctx, i.GuildID)
	respondEphemeral(s, i, fmt.Sprintf("✅ Birthday announcements will be sent at %02d:00 (in each user's timezone)", hour))
}

// handleBdsetMsgWithYear sets the birthday message with year from command
func (b *Bot) handleBdsetMsgWithYear(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	message := opts[0].StringValue()

	ctx := context.Background()
	if err := b.repo.UpdateGuildMessageWithYear(ctx, i.GuildID, message); err != nil {
		respondError(s, i, "Failed to update message")
		return
	}

	b.checkSetupComplete(ctx, i.GuildID)

	// Show preview
	preview := formatMessage(message, i.Member.User.Username, i.Member.User.ID, intPtr(25))
	respondEphemeral(s, i, fmt.Sprintf("✅ Message updated! Preview:\n> %s", preview))
}

// handleBdsetMsgWithoutYear sets the birthday message without year from command
func (b *Bot) handleBdsetMsgWithoutYear(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	message := opts[0].StringValue()

	ctx := context.Background()
	if err := b.repo.UpdateGuildMessageWithoutYear(ctx, i.GuildID, message); err != nil {
		respondError(s, i, "Failed to update message")
		return
	}

	b.checkSetupComplete(ctx, i.GuildID)

	// Show preview
	preview := formatMessage(message, i.Member.User.Username, i.Member.User.ID, nil)
	respondEphemeral(s, i, fmt.Sprintf("✅ Message updated! Preview:\n> %s", preview))
}

// handleBdsetRoleMention toggles role mention permission
func (b *Bot) handleBdsetRoleMention(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	enabled := opts[0].BoolValue()

	ctx := context.Background()
	if err := b.repo.UpdateGuildRoleMention(ctx, i.GuildID, enabled); err != nil {
		respondError(s, i, "Failed to update setting")
		return
	}

	status := "disabled"
	if enabled {
		status = "enabled"
	}
	respondEphemeral(s, i, fmt.Sprintf("✅ Role mentions in birthday messages are now %s", status))
}

// handleBdsetRequiredRole sets the required role for birthday announcements
func (b *Bot) handleBdsetRequiredRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	
	ctx := context.Background()
	var roleID *string
	
	if len(opts) > 0 {
		id := opts[0].RoleValue(s, i.GuildID).ID
		roleID = &id
		if err := b.repo.UpdateGuildRequiredRole(ctx, i.GuildID, roleID); err != nil {
			respondError(s, i, "Failed to update setting")
			return
		}
		respondEphemeral(s, i, fmt.Sprintf("✅ Users must have <@&%s> to have their birthday announced", id))
	} else {
		if err := b.repo.UpdateGuildRequiredRole(ctx, i.GuildID, nil); err != nil {
			respondError(s, i, "Failed to update setting")
			return
		}
		respondEphemeral(s, i, "✅ Required role removed. All users can have their birthday announced.")
	}
}

// handleBdsetDefaultTimezone sets the default timezone
func (b *Bot) handleBdsetDefaultTimezone(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	tz := opts[0].StringValue()

	if !timezone.ValidateTimezone(tz) {
		respondError(s, i, "Invalid timezone. Please select from the autocomplete list.")
		return
	}

	ctx := context.Background()
	if err := b.repo.UpdateGuildDefaultTimezone(ctx, i.GuildID, tz); err != nil {
		respondError(s, i, "Failed to update setting")
		return
	}

	formatSettings := b.GetFormatSettings(ctx, i.GuildID)
	currentTime, _ := timezone.GetCurrentTime(tz)
	timeDisplay := FormatTime(currentTime, formatSettings)
	respondEphemeral(s, i, fmt.Sprintf("✅ Default timezone set to %s (current time: %s)", tz, timeDisplay))
}

// handleBdsetForce opens modal for force-setting a user's birthday
func (b *Bot) handleBdsetForce(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	user := opts[0].UserValue(s)

	modal := discordgo.InteractionResponseData{
		CustomID: "bdset_force_modal:" + user.ID,
		Title:    fmt.Sprintf("Set Birthday for %s", user.Username),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID:    "birthday_date",
						Label:       "Birthday (e.g., 9/24 or September 24, 2002)",
						Style:       discordgo.TextInputShort,
						Placeholder: "Month/Day or Month Day, Year",
						Required:    true,
						MaxLength:   50,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID:    "birthday_timezone",
						Label:       "Timezone (IANA format)",
						Style:       discordgo.TextInputShort,
						Placeholder: "America/Detroit",
						Required:    false,
						MaxLength:   50,
						Value:       "UTC",
					},
				},
			},
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &modal,
	})
}

// handleBdsetSettings shows current settings
func (b *Bot) handleBdsetSettings(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	gs, err := b.repo.GetGuildSettings(ctx, i.GuildID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondEphemeral(s, i, "No settings configured yet. Use `/bdset interactive` to start setup.")
			return
		}
		respondError(s, i, "Failed to fetch settings")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "🎂 Birthday Bot Settings",
		Color: 0xFF69B4,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Channel",
				Value:  formatChannelSetting(gs.ChannelID),
				Inline: true,
			},
			{
				Name:   "Role",
				Value:  formatRoleSetting(gs.RoleID),
				Inline: true,
			},
			{
				Name:   "Announcement Hour",
				Value:  fmt.Sprintf("%02d:00", gs.TimeUTC),
				Inline: true,
			},
			{
				Name:   "Default Timezone",
				Value:  gs.DefaultTimezone,
				Inline: true,
			},
			{
				Name:   "Role Mentions",
				Value:  formatBool(gs.AllowRoleMention),
				Inline: true,
			},
			{
				Name:   "Required Role",
				Value:  formatRoleSetting(gs.RequiredRoleID),
				Inline: true,
			},
			{
				Name:   "Date Format",
				Value:  formatDateFormatSetting(gs.EuropeanDateFormat),
				Inline: true,
			},
			{
				Name:   "Time Format",
				Value:  formatTimeFormatSetting(gs.Use24hTime),
				Inline: true,
			},
			{
				Name:   "Setup Complete",
				Value:  formatBool(gs.SetupComplete),
				Inline: true,
			},
		},
	}

	if gs.MessageWithYear != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Message (with age)",
			Value: "```" + gs.MessageWithYear + "```",
		})
	}
	if gs.MessageWithoutYear != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Message (without age)",
			Value: "```" + gs.MessageWithoutYear + "```",
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// handleBdsetStop clears all settings
func (b *Bot) handleBdsetStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "⚠️ Are you sure you want to clear all birthday settings? This will stop birthday messages and role assignments.",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Yes, clear settings",
							Style:    discordgo.DangerButton,
							CustomID: "bdset_stop_confirm",
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.SecondaryButton,
							CustomID: "bdset_stop_cancel",
						},
					},
				},
			},
		},
	})
}

// handleBdsetInteractive starts the interactive setup wizard
func (b *Bot) handleBdsetInteractive(s *discordgo.Session, i *discordgo.InteractionCreate) {
	modal := discordgo.InteractionResponseData{
		CustomID: "bdset_interactive_modal",
		Title:    "Birthday Bot Setup",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID:    "msg_with_year",
						Label:       "Message with age ({mention}, {name}, {new_age})",
						Style:       discordgo.TextInputParagraph,
						Placeholder: "{mention} has turned {new_age}, happy birthday!",
						Required:    true,
						MaxLength:   500,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID:    "msg_without_year",
						Label:       "Message without age ({mention}, {name})",
						Style:       discordgo.TextInputParagraph,
						Placeholder: "Happy birthday {mention}!",
						Required:    true,
						MaxLength:   500,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID:    "time_hour",
						Label:       "Announcement hour (0-23)",
						Style:       discordgo.TextInputShort,
						Placeholder: "0",
						Required:    true,
						MaxLength:   2,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID:    "date_format",
						Label:       "European date format DD/MM? (yes/no)",
						Style:       discordgo.TextInputShort,
						Placeholder: "no",
						Required:    false,
						MaxLength:   3,
						Value:       "no",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID:    "time_format",
						Label:       "24-hour time? (yes/no)",
						Style:       discordgo.TextInputShort,
						Placeholder: "no",
						Required:    false,
						MaxLength:   3,
						Value:       "no",
					},
				},
			},
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &modal,
	})
}

// handleBdsetDateFormat sets European date format preference
func (b *Bot) handleBdsetDateFormat(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	european := opts[0].BoolValue()

	ctx := context.Background()
	if err := b.repo.UpdateGuildEuropeanDateFormat(ctx, i.GuildID, european); err != nil {
		respondError(s, i, "Failed to update setting")
		return
	}

	if european {
		respondEphemeral(s, i, "✅ Date format set to European style (DD/MM/YYYY)")
	} else {
		respondEphemeral(s, i, "✅ Date format set to American style (MM/DD/YYYY)")
	}
}

// handleBdsetTimeFormat sets 24-hour time format preference
func (b *Bot) handleBdsetTimeFormat(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options
	use24h := opts[0].BoolValue()

	ctx := context.Background()
	if err := b.repo.UpdateGuildUse24hTime(ctx, i.GuildID, use24h); err != nil {
		respondError(s, i, "Failed to update setting")
		return
	}

	if use24h {
		respondEphemeral(s, i, "✅ Time format set to 24-hour (e.g., 14:00)")
	} else {
		respondEphemeral(s, i, "✅ Time format set to 12-hour (e.g., 2:00 PM)")
	}
}

// handleBdsetImport imports birthday data from RedBot Birthday cog JSON
func (b *Bot) handleBdsetImport(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check if user is the bot owner (application owner)
	app, err := s.Application("@me")
	if err != nil {
		slog.Error("Failed to get application info", "error", err)
		respondError(s, i, "Failed to verify permissions")
		return
	}
	
	if i.Member.User.ID != app.Owner.ID {
		respondError(s, i, "This command is only available to the bot owner")
		return
	}

	opts := i.ApplicationCommandData().Options[0].Options
	
	// Get attachment ID from option
	attachmentID := opts[0].Value.(string)
	
	// Get attachment from resolved data
	attachment, ok := i.ApplicationCommandData().Resolved.Attachments[attachmentID]
	if !ok {
		respondError(s, i, "Could not find the attached file")
		return
	}

	// Verify it's a JSON file
	if attachment.ContentType != "application/json" && !strings.HasSuffix(attachment.Filename, ".json") {
		respondError(s, i, "Please attach a JSON file")
		return
	}

	slog.Info("Starting birthday import", "guild_id", i.GuildID, "filename", attachment.Filename, "size", attachment.Size)

	// Download the file
	resp, err := http.Get(attachment.URL)
	if err != nil {
		slog.Error("Failed to download file", "error", err)
		respondError(s, i, "Failed to download file: "+err.Error())
		return
	}
	defer resp.Body.Close()

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read file", "error", err)
		respondError(s, i, "Failed to read file: "+err.Error())
		return
	}

	// Parse the JSON - RedBot cog format
	var cogData map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &cogData); err != nil {
		slog.Error("Failed to parse JSON", "error", err)
		respondError(s, i, "Failed to parse JSON: "+err.Error())
		return
	}

	// The top level has a cog ID, we need to find the data inside it
	var rootData struct {
		Global json.RawMessage `json:"GLOBAL"`
		Guild  map[string]struct {
			TimeUTC          int    `json:"time_utc_s"`
			MessageWithYear  string `json:"message_w_year"`
			MessageWithoutYear string `json:"message_wo_year"`
			ChannelID        int64  `json:"channel_id"`
			RoleID           int64  `json:"role_id"`
			SetupState       int    `json:"setup_state"`
			RequireRole      int64  `json:"require_role"`
			AllowRoleMention bool   `json:"allow_role_mention"`
		} `json:"GUILD"`
		Member map[string]map[string]struct {
			Birthday struct {
				Year  *int `json:"year"`
				Month int  `json:"month"`
				Day   int  `json:"day"`
			} `json:"birthday"`
		} `json:"MEMBER"`
	}

	// Try to find the cog data inside
	var parsedRoot bool
	for _, v := range cogData {
		if err := json.Unmarshal(v, &rootData); err == nil && (rootData.Guild != nil || rootData.Member != nil) {
			parsedRoot = true
			break
		}
	}

	if !parsedRoot {
		respondError(s, i, "Could not find valid birthday cog data in JSON")
		return
	}

	ctx := context.Background()
	
	// Get guild's default timezone (or use UTC)
	defaultTZ := "UTC"
	gs, err := b.repo.GetGuildSettings(ctx, i.GuildID)
	if err == nil && gs != nil && gs.DefaultTimezone != "" {
		defaultTZ = gs.DefaultTimezone
	}

	var importedCount int
	var errorCount int

	// Import member birthdays for this guild
	if rootData.Member != nil {
		for guildID, members := range rootData.Member {
			// Only import for the current guild
			if guildID != i.GuildID {
				continue
			}

			for userID, data := range members {
				var year *int
				if data.Birthday.Year != nil && *data.Birthday.Year > 0 {
					year = data.Birthday.Year
				}

				mb := &database.MemberBirthday{
					GuildID:  i.GuildID,
					UserID:   userID,
					Month:    data.Birthday.Month,
					Day:      data.Birthday.Day,
					Year:     year,
					Timezone: defaultTZ, // Use server's default timezone
				}

				if err := b.repo.SetMemberBirthday(ctx, mb); err != nil {
					slog.Warn("Failed to import birthday", "user_id", userID, "error", err)
					errorCount++
				} else {
					importedCount++
					slog.Debug("Imported birthday", "user_id", userID, "month", data.Birthday.Month, "day", data.Birthday.Day)
				}
			}
		}
	}

	// Import guild settings if they exist for this guild
	if rootData.Guild != nil {
		if guildConfig, exists := rootData.Guild[i.GuildID]; exists {
			channelID := strconv.FormatInt(guildConfig.ChannelID, 10)
			roleID := strconv.FormatInt(guildConfig.RoleID, 10)
			
			importedGS := &database.GuildSettings{
				GuildID:            i.GuildID,
				ChannelID:          &channelID,
				RoleID:             &roleID,
				TimeUTC:            guildConfig.TimeUTC,
				MessageWithYear:    guildConfig.MessageWithYear,
				MessageWithoutYear: guildConfig.MessageWithoutYear,
				AllowRoleMention:   guildConfig.AllowRoleMention,
				DefaultTimezone:    defaultTZ,
				SetupComplete:      guildConfig.SetupState >= 5,
			}
			
			if guildConfig.RequireRole > 0 {
				reqRole := strconv.FormatInt(guildConfig.RequireRole, 10)
				importedGS.RequiredRoleID = &reqRole
			}

			if err := b.repo.UpsertGuildSettings(ctx, importedGS); err != nil {
				slog.Error("Failed to import guild settings", "error", err)
			} else {
				slog.Info("Imported guild settings", "guild_id", i.GuildID)
			}
		}
	}

	respondEphemeral(s, i, fmt.Sprintf("✅ Import complete!\n\n**Birthdays imported:** %d\n**Errors:** %d\n**Timezone used:** %s", importedCount, errorCount, defaultTZ))
}

// checkSetupComplete checks if all required settings are configured
func (b *Bot) checkSetupComplete(ctx context.Context, guildID string) {
	gs, err := b.repo.GetGuildSettings(ctx, guildID)
	if err != nil {
		return
	}

	complete := gs.ChannelID != nil && gs.RoleID != nil && 
		gs.MessageWithYear != "" && gs.MessageWithoutYear != ""

	if complete != gs.SetupComplete {
		b.repo.UpdateGuildSetupComplete(ctx, guildID, complete)
	}
}

// Helper functions

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		slog.Error("Failed to send ephemeral response", "error", err, "content", content)
	}
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		slog.Error("Failed to send error response", "error", err, "message", message)
	}
}

func formatChannelSetting(channelID *string) string {
	if channelID == nil {
		return "Not set"
	}
	return fmt.Sprintf("<#%s>", *channelID)
}

func formatRoleSetting(roleID *string) string {
	if roleID == nil {
		return "Not set"
	}
	return fmt.Sprintf("<@&%s>", *roleID)
}

func formatBool(b bool) string {
	if b {
		return "✅ Enabled"
	}
	return "❌ Disabled"
}

func formatDateFormatSetting(european bool) string {
	if european {
		return "DD/MM/YYYY (European)"
	}
	return "MM/DD/YYYY (American)"
}

func formatTimeFormatSetting(use24h bool) string {
	if use24h {
		return "24-hour (14:00)"
	}
	return "12-hour (2:00 PM)"
}

// handleBdsetAdmin handles /bdset admin subcommand group
func (b *Bot) handleBdsetAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if len(i.ApplicationCommandData().Options[0].Options) == 0 {
		return
	}

	subcommand := i.ApplicationCommandData().Options[0].Options[0].Name

	switch subcommand {
	case "add":
		b.handleBdsetAdminAdd(s, i)
	case "remove":
		b.handleBdsetAdminRemove(s, i)
	case "list":
		b.handleBdsetAdminList(s, i)
	}
}

// handleBdsetAdminAdd adds a user or role as a bot admin
func (b *Bot) handleBdsetAdminAdd(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options[0].Options

	var userID, roleID string
	for _, opt := range opts {
		switch opt.Name {
		case "user":
			userID = opt.UserValue(s).ID
		case "role":
			roleID = opt.RoleValue(s, i.GuildID).ID
		}
	}

	if userID == "" && roleID == "" {
		respondError(s, i, "Please specify a user or role to add as admin")
		return
	}

	ctx := context.Background()

	if userID != "" {
		if err := b.repo.AddBotAdmin(ctx, i.GuildID, userID, "user", i.Member.User.ID); err != nil {
			respondError(s, i, "Failed to add user as admin")
			return
		}
		respondEphemeral(s, i, fmt.Sprintf("✅ <@%s> has been added as a bot admin", userID))
	}

	if roleID != "" {
		if err := b.repo.AddBotAdmin(ctx, i.GuildID, roleID, "role", i.Member.User.ID); err != nil {
			respondError(s, i, "Failed to add role as admin")
			return
		}
		respondEphemeral(s, i, fmt.Sprintf("✅ <@&%s> has been added as a bot admin role", roleID))
	}
}

// handleBdsetAdminRemove removes a user or role from bot admins
func (b *Bot) handleBdsetAdminRemove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options[0].Options[0].Options

	var userID, roleID string
	for _, opt := range opts {
		switch opt.Name {
		case "user":
			userID = opt.UserValue(s).ID
		case "role":
			roleID = opt.RoleValue(s, i.GuildID).ID
		}
	}

	if userID == "" && roleID == "" {
		respondError(s, i, "Please specify a user or role to remove from admins")
		return
	}

	ctx := context.Background()

	if userID != "" {
		if err := b.repo.RemoveBotAdmin(ctx, i.GuildID, userID, "user"); err != nil {
			respondError(s, i, "Failed to remove user from admins")
			return
		}
		respondEphemeral(s, i, fmt.Sprintf("✅ <@%s> has been removed from bot admins", userID))
	}

	if roleID != "" {
		if err := b.repo.RemoveBotAdmin(ctx, i.GuildID, roleID, "role"); err != nil {
			respondError(s, i, "Failed to remove role from admins")
			return
		}
		respondEphemeral(s, i, fmt.Sprintf("✅ <@&%s> has been removed from bot admin roles", roleID))
	}
}

// handleBdsetAdminList lists all bot admins for the guild
func (b *Bot) handleBdsetAdminList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	admins, err := b.repo.GetBotAdmins(ctx, i.GuildID)
	if err != nil {
		respondError(s, i, "Failed to fetch bot admins")
		return
	}

	if len(admins) == 0 {
		respondEphemeral(s, i, "No bot admins configured. Only users with Manage Server permission can use admin commands.")
		return
	}

	var users, roles []string
	for _, admin := range admins {
		if admin.TargetType == "user" {
			users = append(users, fmt.Sprintf("<@%s>", admin.TargetID))
		} else {
			roles = append(roles, fmt.Sprintf("<@&%s>", admin.TargetID))
		}
	}

	embed := &discordgo.MessageEmbed{
		Title: "🔐 Bot Admins",
		Color: 0x00D9FF,
	}

	if len(users) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Users",
			Value: strings.Join(users, "\n"),
		})
	}

	if len(roles) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Roles",
			Value: strings.Join(roles, "\n"),
		})
	}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "Users with Manage Server permission always have admin access",
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}
