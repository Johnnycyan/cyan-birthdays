package timezone

import (
	"strings"
	"time"
)

// TimezoneInfo represents a timezone with display information
type TimezoneInfo struct {
	Abbreviation string // e.g., "EST"
	IANA         string // e.g., "America/Detroit"
	Offset       string // e.g., "UTC-5"
}

// CommonTimezones is a curated list of common timezones
var CommonTimezones = []TimezoneInfo{
	// North America
	{"EST", "America/Detroit", "UTC-5"},
	{"CST", "America/Chicago", "UTC-6"},
	{"MST", "America/Denver", "UTC-7"},
	{"PST", "America/Los_Angeles", "UTC-8"},
	{"AKST", "America/Anchorage", "UTC-9"},
	{"HST", "Pacific/Honolulu", "UTC-10"},
	{"AST", "America/Halifax", "UTC-4"},
	{"NST", "America/St_Johns", "UTC-3:30"},
	
	// Central/South America
	{"BRT", "America/Sao_Paulo", "UTC-3"},
	{"ART", "America/Argentina/Buenos_Aires", "UTC-3"},
	{"COT", "America/Bogota", "UTC-5"},
	{"CLT", "America/Santiago", "UTC-4"},
	{"PET", "America/Lima", "UTC-5"},
	{"VET", "America/Caracas", "UTC-4"},
	{"MEX", "America/Mexico_City", "UTC-6"},
	
	// Europe
	{"GMT", "Europe/London", "UTC+0"},
	{"WET", "Europe/Lisbon", "UTC+0"},
	{"CET", "Europe/Paris", "UTC+1"},
	{"EET", "Europe/Helsinki", "UTC+2"},
	{"MSK", "Europe/Moscow", "UTC+3"},
	{"IST", "Europe/Dublin", "UTC+0"},
	{"BST", "Europe/London", "UTC+1"},
	
	// Asia
	{"IST", "Asia/Kolkata", "UTC+5:30"},
	{"PKT", "Asia/Karachi", "UTC+5"},
	{"ICT", "Asia/Bangkok", "UTC+7"},
	{"WIB", "Asia/Jakarta", "UTC+7"},
	{"SGT", "Asia/Singapore", "UTC+8"},
	{"HKT", "Asia/Hong_Kong", "UTC+8"},
	{"CST", "Asia/Shanghai", "UTC+8"},
	{"JST", "Asia/Tokyo", "UTC+9"},
	{"KST", "Asia/Seoul", "UTC+9"},
	{"PHT", "Asia/Manila", "UTC+8"},
	{"THA", "Asia/Bangkok", "UTC+7"},
	{"MYT", "Asia/Kuala_Lumpur", "UTC+8"},
	{"UAE", "Asia/Dubai", "UTC+4"},
	{"TRT", "Europe/Istanbul", "UTC+3"},
	
	// Oceania
	{"AEST", "Australia/Sydney", "UTC+10"},
	{"ACST", "Australia/Adelaide", "UTC+9:30"},
	{"AWST", "Australia/Perth", "UTC+8"},
	{"NZST", "Pacific/Auckland", "UTC+12"},
	{"FJT", "Pacific/Fiji", "UTC+12"},
	
	// Africa
	{"SAST", "Africa/Johannesburg", "UTC+2"},
	{"EAT", "Africa/Nairobi", "UTC+3"},
	{"WAT", "Africa/Lagos", "UTC+1"},
	{"CAT", "Africa/Harare", "UTC+2"},
	{"EGY", "Africa/Cairo", "UTC+2"},
	
	// UTC
	{"UTC", "UTC", "UTC+0"},
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
		return false, err
	}
	return currentMonth == month && currentDay == day, nil
}

// ShouldAnnounce checks if we should announce now (user's local hour matches target hour)
func ShouldAnnounce(targetHour int, userTimezone string) (bool, error) {
	currentHour, err := GetLocalHour(userTimezone)
	if err != nil {
		return false, err
	}
	return currentHour == targetHour, nil
}
