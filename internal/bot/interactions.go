package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

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

	// Parse the date
	month, day, year, err := parseDate(dateStr)
	if err != nil {
		respondError(s, i, "Invalid date format. Use formats like: 9/24, September 24, or 9/24/2002")
		return
	}

	// Validate timezone
	if tzStr == "" {
		tzStr = "UTC"
	}
	if !timezone.ValidateTimezone(tzStr) {
		respondError(s, i, fmt.Sprintf("Invalid timezone: %s. Use IANA format like America/Detroit", tzStr))
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

	// Parse the date
	month, day, year, err := parseDate(dateStr)
	if err != nil {
		respondError(s, i, "Invalid date format")
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
	ctx := context.Background()
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

	dateDisplay := time.Month(month).String() + " " + strconv.Itoa(day)
	if year != nil {
		dateDisplay += ", " + strconv.Itoa(*year)
	}

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

// parseDate parses various date formats
func parseDate(input string) (month, day int, year *int, err error) {
	input = strings.TrimSpace(input)

	// Try ISO format YYYY-MM-DD first (e.g., 2000-12-07)
	if len(input) >= 8 {
		for _, sep := range []string{"-", "/", "."} {
			parts := strings.Split(input, sep)
			if len(parts) == 3 {
				// Check if first part looks like a year (4 digits, starts with 19 or 20)
				if len(parts[0]) == 4 {
					y, err1 := strconv.Atoi(parts[0])
					m, err2 := strconv.Atoi(parts[1])
					d, err3 := strconv.Atoi(parts[2])
					if err1 == nil && err2 == nil && err3 == nil &&
						y >= 1900 && y <= 2100 && m >= 1 && m <= 12 && d >= 1 && d <= 31 {
						return m, d, &y, nil
					}
				}
			}
		}
	}

	// Try MM/DD/YYYY or MM-DD-YYYY or MM.DD.YYYY format
	for _, sep := range []string{"/", "-", "."} {
		parts := strings.Split(input, sep)
		if len(parts) >= 2 {
			m, err1 := strconv.Atoi(parts[0])
			d, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && m >= 1 && m <= 12 && d >= 1 && d <= 31 {
				if len(parts) == 3 {
					y, err3 := strconv.Atoi(parts[2])
					if err3 == nil {
						if y < 100 {
							y += 2000
						}
						return m, d, &y, nil
					}
				}
				return m, d, nil, nil
			}
		}
	}

	// Try natural language format (e.g., "September 24" or "September 24, 2002")
	months := map[string]int{
		"january": 1, "jan": 1, "february": 2, "feb": 2, "march": 3, "mar": 3,
		"april": 4, "apr": 4, "may": 5, "june": 6, "jun": 6,
		"july": 7, "jul": 7, "august": 8, "aug": 8, "september": 9, "sep": 9, "sept": 9,
		"october": 10, "oct": 10, "november": 11, "nov": 11, "december": 12, "dec": 12,
	}

	// Remove ordinal suffixes only from numbers (e.g., "1st" -> "1", "23rd" -> "23")
	// Using a simple approach: replace common patterns
	cleaned := input
	cleaned = strings.ReplaceAll(cleaned, ",", " ")
	// Remove "of" for patterns like "7th of December"
	cleaned = strings.ReplaceAll(strings.ToLower(cleaned), " of ", " ")
	// Remove "the" for patterns like "the 7th of December"
	if strings.HasPrefix(strings.ToLower(cleaned), "the ") {
		cleaned = cleaned[4:]
	}

	words := strings.Fields(strings.ToLower(cleaned))
	// Process words to remove ordinal suffixes from numbers only
	for i, word := range words {
		// Check if word is a number with ordinal suffix
		if len(word) > 2 {
			suffix := word[len(word)-2:]
			if suffix == "st" || suffix == "nd" || suffix == "rd" || suffix == "th" {
				numPart := word[:len(word)-2]
				if _, err := strconv.Atoi(numPart); err == nil {
					words[i] = numPart
				}
			}
		}
	}

	for i, word := range words {
		if m, ok := months[word]; ok {
			// Try "Month Day [Year]" format (e.g., "December 7, 2000")
			if i+1 < len(words) {
				d, err := strconv.Atoi(words[i+1])
				if err == nil && d >= 1 && d <= 31 {
					if i+2 < len(words) {
						y, err := strconv.Atoi(words[i+2])
						if err == nil {
							if y < 100 {
								y += 2000
							}
							return m, d, &y, nil
						}
					}
					return m, d, nil, nil
				}
			}
			// Try "Day Month [Year]" format (e.g., "7 December 2000")
			if i > 0 {
				d, err := strconv.Atoi(words[i-1])
				if err == nil && d >= 1 && d <= 31 {
					if i+1 < len(words) {
						y, err := strconv.Atoi(words[i+1])
						if err == nil {
							if y < 100 {
								y += 2000
							}
							return m, d, &y, nil
						}
					}
					return m, d, nil, nil
				}
			}
		}
	}

	return 0, 0, nil, fmt.Errorf("could not parse date: %s", input)
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
