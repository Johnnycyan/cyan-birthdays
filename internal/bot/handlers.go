package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	
	var dateStr, tzStr string
	for _, opt := range opts {
		switch opt.Name {
		case "birthday":
			dateStr = opt.StringValue()
		case "timezone":
			tzStr = opt.StringValue()
		}
	}

	// Get default timezone if not provided
	if tzStr == "" {
		ctx := context.Background()
		gs, err := b.repo.GetGuildSettings(ctx, i.GuildID)
		if err == nil && gs != nil {
			tzStr = gs.DefaultTimezone
		} else {
			tzStr = "UTC"
		}
	}

	// Parse the date
	month, day, year, err := parseDate(dateStr)
	if err != nil {
		respondError(s, i, "Invalid date format. Use formats like: 9/24, September 24, or 9/24/2002")
		return
	}

	// Validate timezone
	if !timezone.ValidateTimezone(tzStr) {
		respondError(s, i, fmt.Sprintf("Invalid timezone: %s. Please select from the autocomplete list.", tzStr))
		return
	}

	// Save to database
	ctx := context.Background()
	mb := &database.MemberBirthday{
		GuildID:  i.GuildID,
		UserID:   i.Member.User.ID,
		Month:    month,
		Day:      day,
		Year:     year,
		Timezone: tzStr,
	}
	
	if err := b.repo.SetMemberBirthday(ctx, mb); err != nil {
		slog.Error("Failed to save birthday", "error", err)
		respondError(s, i, "Failed to save your birthday")
		return
	}

	// Format confirmation
	dateDisplay := time.Month(month).String() + " " + strconv.Itoa(day)
	if year != nil {
		dateDisplay += ", " + strconv.Itoa(*year)
	}

	currentTime, _ := timezone.GetCurrentTime(tzStr)
	
	respondEphemeral(s, i, fmt.Sprintf(
		"🎂 Your birthday has been set to **%s**!\nTimezone: %s (current time: %s)",
		dateDisplay, tzStr, currentTime.Format("3:04 PM"),
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

	// Filter to upcoming birthdays
	now := time.Now()
	type upcomingBday struct {
		UserID   string
		Month    int
		Day      int
		Year     *int
		DaysAway int
	}

	var upcoming []upcomingBday
	for _, bd := range birthdays {
		// Calculate days until birthday
		thisYearBday := time.Date(now.Year(), time.Month(bd.Month), bd.Day, 0, 0, 0, 0, time.UTC)
		if thisYearBday.Before(now) {
			thisYearBday = thisYearBday.AddDate(1, 0, 0)
		}
		daysAway := int(thisYearBday.Sub(now).Hours() / 24)
		
		if daysAway <= days {
			upcoming = append(upcoming, upcomingBday{
				UserID:   bd.UserID,
				Month:    bd.Month,
				Day:      bd.Day,
				Year:     bd.Year,
				DaysAway: daysAway,
			})
		}
	}

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
		Color: 0xFF69B4, // Hot pink
	}

	// Group by date
	dateMap := make(map[string][]string)
	for _, bd := range upcoming {
		dateKey := fmt.Sprintf("%s %d", time.Month(bd.Month).String(), bd.Day)
		if bd.DaysAway == 0 {
			dateKey = "Today!"
		} else if bd.DaysAway == 1 {
			dateKey = "Tomorrow"
		}
		
		mention := fmt.Sprintf("<@%s>", bd.UserID)
		if bd.Year != nil && *bd.Year > 0 {
			age := now.Year() - *bd.Year
			if bd.DaysAway > 0 {
				mention += fmt.Sprintf(" (turning %d)", age)
			} else {
				mention += fmt.Sprintf(" (now %d)", age)
			}
		}
		dateMap[dateKey] = append(dateMap[dateKey], mention)
	}

	for date, users := range dateMap {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   date,
			Value:  strings.Join(users, "\n"),
			Inline: true,
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// handleBdsetCommand handles /bdset subcommands
func (b *Bot) handleBdsetCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if len(i.ApplicationCommandData().Options) == 0 {
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

	currentTime, _ := timezone.GetCurrentTime(tz)
	respondEphemeral(s, i, fmt.Sprintf("✅ Default timezone set to %s (current time: %s)", tz, currentTime.Format("3:04 PM")))
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
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
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
