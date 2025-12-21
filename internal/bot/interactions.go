package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Johnnycyan/cyan-birthdays/internal/database"
	"github.com/Johnnycyan/cyan-birthdays/internal/timezone"
	"github.com/bwmarrin/discordgo"
)

// handleAutocomplete handles autocomplete interactions
func (b *Bot) handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	
	// Find the focused option
	var focused *discordgo.ApplicationCommandInteractionDataOption
	for _, opt := range data.Options {
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand {
			for _, subOpt := range opt.Options {
				if subOpt.Focused {
					focused = subOpt
					break
				}
			}
		}
	}

	if focused == nil {
		return
	}

	// Handle timezone autocomplete
	if focused.Name == "timezone" {
		query := focused.StringValue()
		results := timezone.SearchTimezones(query)

		choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(results))
		for _, tz := range results {
			label := timezone.FormatTimezoneChoice(tz)
			// Discord has a 100 character limit for choice names
			if len(label) > 100 {
				label = label[:97] + "..."
			}
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  label,
				Value: tz.IANA,
			})
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		})
	}
}

// handleModalSubmit handles modal submissions
func (b *Bot) handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	
	switch {
	case data.CustomID == "birthday_set_modal":
		b.handleBirthdaySetModal(s, i)
	case data.CustomID == "bdset_msgwithyear_modal":
		b.handleMsgWithYearModal(s, i)
	case data.CustomID == "bdset_msgwithoutyear_modal":
		b.handleMsgWithoutYearModal(s, i)
	case data.CustomID == "bdset_interactive_modal":
		b.handleInteractiveModal(s, i)
	case strings.HasPrefix(data.CustomID, "bdset_force_modal:"):
		b.handleForceModal(s, i)
	}
}

// handleBirthdaySetModal processes birthday set modal submission
func (b *Bot) handleBirthdaySetModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	
	var dateStr, tzStr string
	for _, comp := range data.Components {
		row := comp.(*discordgo.ActionsRow)
		for _, c := range row.Components {
			input := c.(*discordgo.TextInput)
			switch input.CustomID {
			case "birthday_date":
				dateStr = input.Value
			case "birthday_timezone":
				tzStr = input.Value
			}
		}
	}

	ctx := context.Background()
	formatSettings := b.GetFormatSettings(ctx, i.GuildID)

	// Parse the date with format settings
	month, day, year, err := ParseDateWithSettings(dateStr, formatSettings)
	if err != nil {
		formatHint := "MM/DD"
		if formatSettings.EuropeanDateFormat {
			formatHint = "DD/MM"
		}
		respondError(s, i, fmt.Sprintf("Invalid date format. Use formats like: %s, September 24, or %s/2002", formatHint, formatHint))
		return
	}

	// Validate timezone
	if tzStr == "" {
		tzStr = "UTC"
	}
	if !timezone.ValidateTimezone(tzStr) {
		respondError(s, i, fmt.Sprintf("Invalid timezone: %s. Use IANA format like America/New_York", tzStr))
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
	
	if err := b.repo.SetMemberBirthday(ctx, mb); err != nil {
		slog.Error("Failed to save birthday", "error", err)
		respondError(s, i, "Failed to save your birthday")
		return
	}

	// Format confirmation using guild settings
	dateDisplay := FormatDate(month, day, year, formatSettings)
	currentTime, _ := timezone.GetCurrentTime(tzStr)
	timeDisplay := FormatTime(currentTime, formatSettings)
	
	respondEphemeral(s, i, fmt.Sprintf(
		"🎂 Your birthday has been set to **%s**!\nTimezone: %s (current time: %s)",
		dateDisplay, tzStr, timeDisplay,
	))
}

// handleMsgWithYearModal processes message with year modal
func (b *Bot) handleMsgWithYearModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	
	var message string
	for _, comp := range data.Components {
		row := comp.(*discordgo.ActionsRow)
		for _, c := range row.Components {
			input := c.(*discordgo.TextInput)
			if input.CustomID == "message" {
				message = input.Value
			}
		}
	}

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

// handleMsgWithoutYearModal processes message without year modal
func (b *Bot) handleMsgWithoutYearModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	
	var message string
	for _, comp := range data.Components {
		row := comp.(*discordgo.ActionsRow)
		for _, c := range row.Components {
			input := c.(*discordgo.TextInput)
			if input.CustomID == "message" {
				message = input.Value
			}
		}
	}

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

// handleInteractiveModal processes the interactive setup modal
func (b *Bot) handleInteractiveModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	
	var msgWithYear, msgWithoutYear, timeStr, dateFormatStr, timeFormatStr string
	for _, comp := range data.Components {
		row := comp.(*discordgo.ActionsRow)
		for _, c := range row.Components {
			input := c.(*discordgo.TextInput)
			switch input.CustomID {
			case "msg_with_year":
				msgWithYear = input.Value
			case "msg_without_year":
				msgWithoutYear = input.Value
			case "time_hour":
				timeStr = input.Value
			case "date_format":
				dateFormatStr = input.Value
			case "time_format":
				timeFormatStr = input.Value
			}
		}
	}

	// Parse hour
	hour, err := strconv.Atoi(timeStr)
	if err != nil || hour < 0 || hour > 23 {
		respondError(s, i, "Invalid hour. Please enter a number between 0 and 23.")
		return
	}

	// Parse boolean values
	europeanDate := strings.ToLower(strings.TrimSpace(dateFormatStr)) == "yes"
	use24hTime := strings.ToLower(strings.TrimSpace(timeFormatStr)) == "yes"

	ctx := context.Background()
	
	// Update messages
	if err := b.repo.UpdateGuildMessageWithYear(ctx, i.GuildID, msgWithYear); err != nil {
		respondError(s, i, "Failed to update settings")
		return
	}
	if err := b.repo.UpdateGuildMessageWithoutYear(ctx, i.GuildID, msgWithoutYear); err != nil {
		respondError(s, i, "Failed to update settings")
		return
	}
	if err := b.repo.UpdateGuildTime(ctx, i.GuildID, hour); err != nil {
		respondError(s, i, "Failed to update settings")
		return
	}
	if err := b.repo.UpdateGuildEuropeanDateFormat(ctx, i.GuildID, europeanDate); err != nil {
		respondError(s, i, "Failed to update settings")
		return
	}
	if err := b.repo.UpdateGuildUse24hTime(ctx, i.GuildID, use24hTime); err != nil {
		respondError(s, i, "Failed to update settings")
		return
	}

	b.checkSetupComplete(ctx, i.GuildID)

	dateFormatDisplay := "MM/DD/YYYY"
	if europeanDate {
		dateFormatDisplay = "DD/MM/YYYY"
	}
	timeFormatDisplay := "12-hour"
	if use24hTime {
		timeFormatDisplay = "24-hour"
	}

	respondEphemeral(s, i, fmt.Sprintf(
		"✅ Settings saved!\n\n"+
			"**Announcement hour:** %02d:00\n"+
			"**Date format:** %s\n"+
			"**Time format:** %s\n\n"+
			"Now set the channel and role:\n"+
			"• `/bdset channel #channel`\n"+
			"• `/bdset role @role`",
		hour, dateFormatDisplay, timeFormatDisplay,
	))
}

// handleForceModal processes the force-set birthday modal
func (b *Bot) handleForceModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	
	// Extract user ID from custom ID
	parts := strings.Split(data.CustomID, ":")
	if len(parts) != 2 {
		respondError(s, i, "Invalid modal data")
		return
	}
	targetUserID := parts[1]

	var dateStr, tzStr string
	for _, comp := range data.Components {
		row := comp.(*discordgo.ActionsRow)
		for _, c := range row.Components {
			input := c.(*discordgo.TextInput)
			switch input.CustomID {
			case "birthday_date":
				dateStr = input.Value
			case "birthday_timezone":
				tzStr = input.Value
			}
		}
	}

	ctx := context.Background()
	formatSettings := b.GetFormatSettings(ctx, i.GuildID)

	// Parse the date with format settings
	month, day, year, err := ParseDateWithSettings(dateStr, formatSettings)
	if err != nil {
		formatHint := "MM/DD"
		if formatSettings.EuropeanDateFormat {
			formatHint = "DD/MM"
		}
		respondError(s, i, fmt.Sprintf("Invalid date format. Use formats like: %s or %s/2002", formatHint, formatHint))
		return
	}

	// Validate timezone
	if tzStr == "" {
		tzStr = "UTC"
	}
	if !timezone.ValidateTimezone(tzStr) {
		respondError(s, i, fmt.Sprintf("Invalid timezone: %s", tzStr))
		return
	}

	// Save to database
	mb := &database.MemberBirthday{
		GuildID:  i.GuildID,
		UserID:   targetUserID,
		Month:    month,
		Day:      day,
		Year:     year,
		Timezone: tzStr,
	}
	
	if err := b.repo.SetMemberBirthday(ctx, mb); err != nil {
		respondError(s, i, "Failed to save birthday")
		return
	}

	dateDisplay := FormatDate(month, day, year, formatSettings)
	respondEphemeral(s, i, fmt.Sprintf("✅ Birthday for <@%s> set to **%s** (%s)", targetUserID, dateDisplay, tzStr))
}

// handleComponent handles button and select menu interactions
func (b *Bot) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	switch customID {
	case "birthday_remove_confirm":
		ctx := context.Background()
		if err := b.repo.DeleteMemberBirthday(ctx, i.GuildID, i.Member.User.ID); err != nil {
			respondError(s, i, "Failed to remove birthday")
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "✅ Your birthday has been removed.",
				Components: []discordgo.MessageComponent{},
			},
		})

	case "birthday_remove_cancel":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Cancelled.",
				Components: []discordgo.MessageComponent{},
			},
		})

	case "bdset_stop_confirm":
		ctx := context.Background()
		if err := b.repo.ClearGuildSettings(ctx, i.GuildID); err != nil {
			respondError(s, i, "Failed to clear settings")
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "✅ All birthday settings have been cleared.",
				Components: []discordgo.MessageComponent{},
			},
		})

	case "bdset_stop_cancel":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Cancelled.",
				Components: []discordgo.MessageComponent{},
			},
		})
	}
}

// formatMessage replaces placeholders in a birthday message
func formatMessage(template, name, userID string, age *int) string {
	result := template
	result = strings.ReplaceAll(result, "{mention}", "<@"+userID+">")
	result = strings.ReplaceAll(result, "{name}", name)
	if age != nil {
		result = strings.ReplaceAll(result, "{new_age}", strconv.Itoa(*age))
	}
	return result
}

func intPtr(i int) *int {
	return &i
}
