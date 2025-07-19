package core

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type UserPreferences struct {
	Name         string
	Timezone     string
	PromptTime   time.Time
	ProjectFocus *string
}

func parseUserPreferences(body string) (*UserPreferences, error) {
	prefs := &UserPreferences{}
	
	// Extract name
	nameRegex := regexp.MustCompile(`(?i)name:\s*([^\n\r]+)`)
	if matches := nameRegex.FindStringSubmatch(body); len(matches) > 1 {
		prefs.Name = strings.TrimSpace(matches[1])
	}
	
	// Extract timezone
	timezoneRegex := regexp.MustCompile(`(?i)timezone[^:]*:\s*([^\n\r]+)`)
	if matches := timezoneRegex.FindStringSubmatch(body); len(matches) > 1 {
		prefs.Timezone = strings.TrimSpace(matches[1])
	}
	
	// Extract prompt time
	timeRegex := regexp.MustCompile(`(?i)(?:time|prompt)[^:]*:\s*([^\n\r]+)`)
	if matches := timeRegex.FindStringSubmatch(body); len(matches) > 1 {
		timeStr := strings.TrimSpace(matches[1])
		parsedTime, err := parseTimeString(timeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid time format: %s", timeStr)
		}
		prefs.PromptTime = parsedTime
	}
	
	// Extract project focus (optional)
	projectRegex := regexp.MustCompile(`(?i)(?:project|focus)[^:]*:\s*([^\n\r]+)`)
	if matches := projectRegex.FindStringSubmatch(body); len(matches) > 1 {
		projectName := strings.TrimSpace(matches[1])
		if projectName != "" && projectName != "_" && projectName != "___________" {
			prefs.ProjectFocus = &projectName
		}
	}
	
	// Validate required fields
	if prefs.Name == "" || prefs.Name == "_" || prefs.Name == "___________" {
		return nil, fmt.Errorf("name is required")
	}
	
	if prefs.Timezone == "" || prefs.Timezone == "_" || prefs.Timezone == "___________" {
		return nil, fmt.Errorf("timezone is required")
	}
	
	if prefs.PromptTime.IsZero() {
		// Default to 4 PM if not specified
		prefs.PromptTime = time.Date(0, 1, 1, 16, 0, 0, 0, time.UTC)
	}
	
	// Validate timezone
	if !isValidTimezone(prefs.Timezone) {
		return nil, fmt.Errorf("invalid timezone: %s", prefs.Timezone)
	}
	
	return prefs, nil
}

func parseTimeString(timeStr string) (time.Time, error) {
	// Common time formats
	formats := []string{
		"15:04",     // 16:00
		"3:04 PM",   // 4:00 PM
		"3:04PM",    // 4:00PM
		"3 PM",      // 4 PM
		"3PM",       // 4PM
		"15",        // 16
	}
	
	timeStr = strings.TrimSpace(timeStr)
	
	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return time.Date(0, 1, 1, t.Hour(), t.Minute(), 0, 0, time.UTC), nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

func isValidTimezone(tz string) bool {
	// Common timezone validation
	validTimezones := []string{
		"UTC", "GMT",
		"America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
		"America/Toronto", "America/Vancouver", "America/Montreal",
		"Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Rome", "Europe/Madrid",
		"Asia/Tokyo", "Asia/Shanghai", "Asia/Kolkata", "Asia/Dubai",
		"Australia/Sydney", "Australia/Melbourne",
		"Pacific/Auckland",
	}
	
	for _, valid := range validTimezones {
		if strings.EqualFold(tz, valid) {
			return true
		}
	}
	
	// Try to load the timezone to validate it
	_, err := time.LoadLocation(tz)
	return err == nil
}