package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Johnnycyan/cyan-birthdays/internal/database"
)

// FormatSettings holds the formatting preferences for a guild
type FormatSettings struct {
	EuropeanDateFormat bool
	Use24hTime         bool
}

// GetFormatSettings retrieves format settings for a guild
func (b *Bot) GetFormatSettings(ctx context.Context, guildID string) FormatSettings {
	gs, err := b.repo.GetGuildSettings(ctx, guildID)
	if err != nil || gs == nil {
		return FormatSettings{} // Default to American format
	}
	return FormatSettings{
		EuropeanDateFormat: gs.EuropeanDateFormat,
		Use24hTime:         gs.Use24hTime,
	}
}

// FormatTime formats a time according to guild settings
func FormatTime(t time.Time, settings FormatSettings) string {
	if settings.Use24hTime {
		return t.Format("15:04")
	}
	return t.Format("3:04 PM")
}

// FormatDate formats a date according to guild settings
func FormatDate(month, day int, year *int, settings FormatSettings) string {
	var dateDisplay string
	if settings.EuropeanDateFormat {
		// European: Day Month
		dateDisplay = fmt.Sprintf("%d %s", day, time.Month(month).String())
	} else {
		// American: Month Day
		dateDisplay = time.Month(month).String() + " " + strconv.Itoa(day)
	}
	if year != nil {
		dateDisplay += ", " + strconv.Itoa(*year)
	}
	return dateDisplay
}

// ParseDateWithSettings parses a date string according to guild settings
// Returns month, day, year (optional)
func ParseDateWithSettings(input string, settings FormatSettings) (month, day int, year *int, err error) {
	input = strings.TrimSpace(input)

	// Try ISO format YYYY-MM-DD first (e.g., 2000-12-07)
	// ISO is always YYYY-MM-DD regardless of settings
	if len(input) >= 8 {
		for _, sep := range []string{"-", "/", "."} {
			parts := strings.Split(input, sep)
			if len(parts) == 3 {
				// Check if first part looks like a year (4 digits)
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

	// Try numeric format with separators
	for _, sep := range []string{"/", "-", "."} {
		parts := strings.Split(input, sep)
		if len(parts) >= 2 {
			first, err1 := strconv.Atoi(parts[0])
			second, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				var m, d int
				if settings.EuropeanDateFormat {
					// European: DD/MM/YYYY
					d, m = first, second
				} else {
					// American: MM/DD/YYYY
					m, d = first, second
				}

				if m >= 1 && m <= 12 && d >= 1 && d <= 31 {
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
	}

	// Try natural language format
	months := map[string]int{
		"january": 1, "jan": 1, "february": 2, "feb": 2, "march": 3, "mar": 3,
		"april": 4, "apr": 4, "may": 5, "june": 6, "jun": 6,
		"july": 7, "jul": 7, "august": 8, "aug": 8, "september": 9, "sep": 9, "sept": 9,
		"october": 10, "oct": 10, "november": 11, "nov": 11, "december": 12, "dec": 12,
	}

	// Clean up input
	cleaned := input
	cleaned = strings.ReplaceAll(cleaned, ",", " ")
	cleaned = strings.ReplaceAll(strings.ToLower(cleaned), " of ", " ")
	if strings.HasPrefix(strings.ToLower(cleaned), "the ") {
		cleaned = cleaned[4:]
	}

	words := strings.Fields(strings.ToLower(cleaned))
	// Process words to remove ordinal suffixes from numbers only
	for i, word := range words {
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

// Ensure we use the database package
var _ = database.GuildSettings{}
