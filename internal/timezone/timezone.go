package timezone

import (
	"log/slog"
	"strings"
	"time"
)

// TimezoneInfo represents a timezone with display information
type TimezoneInfo struct {
	Abbreviation string // e.g., "EST"
	IANA         string // e.g., "America/Detroit"
	Offset       string // e.g., "UTC-5"
}

// CommonTimezones is a curated list covering all UTC offsets (no duplicates)
var CommonTimezones = []TimezoneInfo{
	// UTC-12 to UTC-1
	{"AoE", "Etc/GMT+12", "UTC-12"},
	{"SST", "Pacific/Pago_Pago", "UTC-11"},
	{"HST", "Pacific/Honolulu", "UTC-10"},
	{"HDT", "Pacific/Marquesas", "UTC-9:30"},
	{"AKST", "America/Anchorage", "UTC-9"},
	{"PST", "America/Los_Angeles", "UTC-8"},
	{"MST", "America/Denver", "UTC-7"},
	{"CST", "America/Chicago", "UTC-6"},
	{"EST", "America/New_York", "UTC-5"},
	{"AST", "America/Halifax", "UTC-4"},
	{"NST", "America/St_Johns", "UTC-3:30"},
	{"BRT", "America/Sao_Paulo", "UTC-3"},
	{"GST", "Atlantic/South_Georgia", "UTC-2"},
	{"AZOT", "Atlantic/Azores", "UTC-1"},

	// UTC+0
	{"UTC", "UTC", "UTC+0"},

	// UTC+1 to UTC+14
	{"CET", "Europe/Paris", "UTC+1"},
	{"EET", "Europe/Helsinki", "UTC+2"},
	{"MSK", "Europe/Moscow", "UTC+3"},
	{"IRST", "Asia/Tehran", "UTC+3:30"},
	{"GST", "Asia/Dubai", "UTC+4"},
	{"AFT", "Asia/Kabul", "UTC+4:30"},
	{"PKT", "Asia/Karachi", "UTC+5"},
	{"IST", "Asia/Kolkata", "UTC+5:30"},
	{"NPT", "Asia/Kathmandu", "UTC+5:45"},
	{"BST", "Asia/Dhaka", "UTC+6"},
	{"MMT", "Asia/Yangon", "UTC+6:30"},
	{"ICT", "Asia/Bangkok", "UTC+7"},
	{"CST", "Asia/Shanghai", "UTC+8"},
	{"ACWST", "Australia/Eucla", "UTC+8:45"},
	{"JST", "Asia/Tokyo", "UTC+9"},
	{"ACST", "Australia/Adelaide", "UTC+9:30"},
	{"AEST", "Australia/Sydney", "UTC+10"},
	{"LHST", "Australia/Lord_Howe", "UTC+10:30"},
	{"SBT", "Pacific/Guadalcanal", "UTC+11"},
	{"NZST", "Pacific/Auckland", "UTC+12"},
	{"CHAST", "Pacific/Chatham", "UTC+12:45"},
	{"TOT", "Pacific/Tongatapu", "UTC+13"},
	{"LINT", "Pacific/Kiritimati", "UTC+14"},
}

// GetCurrentTime returns the current time in the given timezone
func GetCurrentTime(ianaName string) (time.Time, error) {
	loc, err := time.LoadLocation(ianaName)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().In(loc), nil
}

// FormatTimezoneChoice formats a timezone for the Discord autocomplete
func FormatTimezoneChoice(tz TimezoneInfo) string {
	currentTime, err := GetCurrentTime(tz.IANA)
	if err != nil {
		return tz.Abbreviation + " - " + tz.IANA + " - " + tz.Offset
	}
	timeStr := currentTime.Format("3:04 PM")
	return tz.Abbreviation + " - " + tz.IANA + " - " + tz.Offset + " - " + timeStr
}

// SearchTimezones filters timezones based on a search query
func SearchTimezones(query string) []TimezoneInfo {
	if query == "" {
		// Return first 25 (Discord limit)
		if len(CommonTimezones) > 25 {
			return CommonTimezones[:25]
		}
		return CommonTimezones
	}

	query = strings.ToLower(query)
	var results []TimezoneInfo

	for _, tz := range CommonTimezones {
		if strings.Contains(strings.ToLower(tz.Abbreviation), query) ||
			strings.Contains(strings.ToLower(tz.IANA), query) ||
			strings.Contains(strings.ToLower(tz.Offset), query) {
			results = append(results, tz)
		}
	}

	// Limit to 25 (Discord autocomplete limit)
	if len(results) > 25 {
		return results[:25]
	}
	return results
}

// ValidateTimezone checks if a timezone is valid
func ValidateTimezone(ianaName string) bool {
	_, err := time.LoadLocation(ianaName)
	return err == nil
}

// GetOffset returns the UTC offset for a timezone at the current time
func GetOffset(ianaName string) (int, error) {
	loc, err := time.LoadLocation(ianaName)
	if err != nil {
		return 0, err
	}
	_, offset := time.Now().In(loc).Zone()
	return offset / 3600, nil // Return hours
}

// GetLocalHour returns the current hour in the given timezone
func GetLocalHour(ianaName string) (int, error) {
	loc, err := time.LoadLocation(ianaName)
	if err != nil {
		return 0, err
	}
	return time.Now().In(loc).Hour(), nil
}

// GetLocalDate returns the current month and day in the given timezone
func GetLocalDate(ianaName string) (month, day int, err error) {
	loc, err := time.LoadLocation(ianaName)
	if err != nil {
		return 0, 0, err
	}
	now := time.Now().In(loc)
	return int(now.Month()), now.Day(), nil
}

// IsBirthdayToday checks if a birthday (month/day) is today in the given timezone
func IsBirthdayToday(month, day int, ianaName string) (bool, error) {
	currentMonth, currentDay, err := GetLocalDate(ianaName)
	if err != nil {
		slog.Debug("IsBirthdayToday error", "timezone", ianaName, "error", err)
		return false, err
	}
	result := currentMonth == month && currentDay == day
	slog.Debug("IsBirthdayToday", "timezone", ianaName, "current", currentMonth*100+currentDay, "birthday", month*100+day, "result", result)
	return result, nil
}

// ShouldAnnounce checks if we should announce now (user's local hour matches target hour)
func ShouldAnnounce(targetHour int, userTimezone string) (bool, error) {
	currentHour, err := GetLocalHour(userTimezone)
	if err != nil {
		slog.Debug("ShouldAnnounce error", "timezone", userTimezone, "error", err)
		return false, err
	}
	result := currentHour == targetHour
	slog.Debug("ShouldAnnounce", "timezone", userTimezone, "currentHour", currentHour, "targetHour", targetHour, "result", result)
	return result, nil
}
