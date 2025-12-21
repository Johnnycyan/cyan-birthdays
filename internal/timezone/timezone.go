package timezone

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/zlasd/tzloc"
)

// TimezoneInfo represents a timezone with display information
type TimezoneInfo struct {
	IANA   string // e.g., "America/New_York"
	Offset string // e.g., "UTC-5" (dynamically calculated)
}

// PopularTimezones are shown first when no search query is provided
var PopularTimezones = []string{
	"America/New_York",
	"America/Chicago",
	"America/Denver",
	"America/Los_Angeles",
	"America/Toronto",
	"America/Vancouver",
	"Europe/London",
	"Europe/Paris",
	"Europe/Berlin",
	"Europe/Amsterdam",
	"Europe/Moscow",
	"Asia/Tokyo",
	"Asia/Shanghai",
	"Asia/Hong_Kong",
	"Asia/Singapore",
	"Asia/Seoul",
	"Asia/Kolkata",
	"Asia/Dubai",
	"Australia/Sydney",
	"Australia/Melbourne",
	"Australia/Perth",
	"Pacific/Auckland",
	"Pacific/Honolulu",
	"UTC",
}

// GetCurrentTime returns the current time in the given timezone
func GetCurrentTime(ianaName string) (time.Time, error) {
	loc, err := time.LoadLocation(ianaName)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().In(loc), nil
}

// formatOffset formats seconds offset into a readable string like "UTC-5" or "UTC+5:30"
func formatOffset(offsetSeconds int) string {
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}

	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60

	if minutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, hours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, hours, minutes)
}

// GetTimezoneInfo creates a TimezoneInfo with dynamically calculated offset
func GetTimezoneInfo(ianaName string) TimezoneInfo {
	loc, err := time.LoadLocation(ianaName)
	if err != nil {
		return TimezoneInfo{IANA: ianaName, Offset: "UTC"}
	}

	_, offsetSeconds := time.Now().In(loc).Zone()
	return TimezoneInfo{
		IANA:   ianaName,
		Offset: formatOffset(offsetSeconds),
	}
}

// FormatTimezoneChoice formats a timezone for the Discord autocomplete
func FormatTimezoneChoice(tz TimezoneInfo) string {
	currentTime, err := GetCurrentTime(tz.IANA)
	if err != nil {
		return tz.IANA + " (" + tz.Offset + ")"
	}
	timeStr := currentTime.Format("3:04 PM")
	return tz.IANA + " (" + tz.Offset + ") - " + timeStr
}

// SearchTimezones filters timezones based on a search query
func SearchTimezones(query string) []TimezoneInfo {
	if query == "" {
		// Return popular timezones when no search query
		results := make([]TimezoneInfo, 0, len(PopularTimezones))
		for _, iana := range PopularTimezones {
			results = append(results, GetTimezoneInfo(iana))
		}
		// Limit to 25 (Discord limit)
		if len(results) > 25 {
			return results[:25]
		}
		return results
	}

	query = strings.ToLower(query)
	allLocations := tzloc.GetLocationList()

	var results []TimezoneInfo

	// Search through all IANA timezones
	for _, iana := range allLocations {
		if strings.Contains(strings.ToLower(iana), query) {
			results = append(results, GetTimezoneInfo(iana))
		}
	}

	// Also search by offset (e.g., "utc-5", "+5:30")
	for _, iana := range allLocations {
		tzInfo := GetTimezoneInfo(iana)
		if strings.Contains(strings.ToLower(tzInfo.Offset), query) {
			// Avoid duplicates
			found := false
			for _, r := range results {
				if r.IANA == iana {
					found = true
					break
				}
			}
			if !found {
				results = append(results, tzInfo)
			}
		}
	}

	// Sort by IANA name for consistency
	sort.Slice(results, func(i, j int) bool {
		return results[i].IANA < results[j].IANA
	})

	// Limit to 25 (Discord autocomplete limit)
	if len(results) > 25 {
		return results[:25]
	}
	return results
}

// ValidateTimezone checks if a timezone is valid
func ValidateTimezone(ianaName string) bool {
	return tzloc.ValidLocation(ianaName)
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
