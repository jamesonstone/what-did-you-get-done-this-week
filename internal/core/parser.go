package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ParsedReply struct {
	Content     string
	Commands    []Command
	ProjectTag  *string
	IsValidated bool
	Error       error
}

type Command struct {
	Type     string
	Value    string
	Duration *time.Duration
}

const (
	CommandTypePause   = "pause"
	CommandTypeProject = "project"
	CommandTypeEntry   = "entry"
)

var (
	pauseRegex   = regexp.MustCompile(`<pause>([^<]+)</pause>`)
	projectRegex = regexp.MustCompile(`<project>([^<]+)</project>`)
	entryRegex   = regexp.MustCompile(`<entry>([^<]+)</entry>`)
)

func ParseEmailReply(rawContent string) *ParsedReply {
	content := strings.TrimSpace(rawContent)
	
	// Remove email signatures and quoted text
	content = cleanEmailContent(content)
	
	result := &ParsedReply{
		Content:     content,
		Commands:    []Command{},
		IsValidated: true,
	}

	// Extract pause commands
	pauseMatches := pauseRegex.FindAllStringSubmatch(content, -1)
	for _, match := range pauseMatches {
		if len(match) > 1 {
			duration, err := parsePauseDuration(match[1])
			if err != nil {
				result.Error = fmt.Errorf("invalid pause duration: %s", match[1])
				result.IsValidated = false
				return result
			}
			
			result.Commands = append(result.Commands, Command{
				Type:     CommandTypePause,
				Value:    match[1],
				Duration: &duration,
			})
		}
	}

	// Extract project commands
	projectMatches := projectRegex.FindAllStringSubmatch(content, -1)
	for _, match := range projectMatches {
		if len(match) > 1 {
			projectName := strings.TrimSpace(match[1])
			result.Commands = append(result.Commands, Command{
				Type:  CommandTypeProject,
				Value: projectName,
			})
			result.ProjectTag = &projectName
		}
	}

	// Extract entry commands (explicit entries)
	entryMatches := entryRegex.FindAllStringSubmatch(content, -1)
	for _, match := range entryMatches {
		if len(match) > 1 {
			entryContent := strings.TrimSpace(match[1])
			result.Commands = append(result.Commands, Command{
				Type:  CommandTypeEntry,
				Value: entryContent,
			})
		}
	}

	// Remove command tags from content
	result.Content = pauseRegex.ReplaceAllString(result.Content, "")
	result.Content = projectRegex.ReplaceAllString(result.Content, "")
	result.Content = entryRegex.ReplaceAllString(result.Content, "")
	result.Content = strings.TrimSpace(result.Content)

	// If no explicit entry and no commands, treat the whole content as an entry
	if result.Content != "" && len(result.Commands) == 0 {
		result.Commands = append(result.Commands, Command{
			Type:  CommandTypeEntry,
			Value: result.Content,
		})
	}

	// Validate that we have at least some meaningful content
	if result.Content == "" && len(result.Commands) == 0 {
		result.Error = fmt.Errorf("no meaningful content found in reply")
		result.IsValidated = false
	}

	return result
}

func parsePauseDuration(durationStr string) (time.Duration, error) {
	durationStr = strings.ToLower(strings.TrimSpace(durationStr))
	
	// Handle common phrases
	switch durationStr {
	case "today":
		return 24 * time.Hour, nil
	case "tomorrow":
		return 24 * time.Hour, nil
	case "this week", "1 week":
		return 7 * 24 * time.Hour, nil
	case "next week":
		return 7 * 24 * time.Hour, nil
	case "this month", "1 month":
		return 30 * 24 * time.Hour, nil
	case "next month":
		return 30 * 24 * time.Hour, nil
	}

	// Try to parse number + unit
	re := regexp.MustCompile(`(\d+)\s*(day|days|week|weeks|month|months)`)
	matches := re.FindStringSubmatch(durationStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid duration format: %s", durationStr)
	}

	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration: %s", matches[1])
	}

	unit := matches[2]
	switch unit {
	case "day", "days":
		return time.Duration(number) * 24 * time.Hour, nil
	case "week", "weeks":
		return time.Duration(number) * 7 * 24 * time.Hour, nil
	case "month", "months":
		return time.Duration(number) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}
}

func cleanEmailContent(content string) string {
	lines := strings.Split(content, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines
		if line == "" {
			continue
		}
		
		// Skip common email signature patterns
		if strings.HasPrefix(line, "--") ||
		   strings.HasPrefix(line, "Sent from") ||
		   strings.HasPrefix(line, "From:") ||
		   strings.HasPrefix(line, "To:") ||
		   strings.HasPrefix(line, "Subject:") ||
		   strings.HasPrefix(line, "Date:") ||
		   strings.HasPrefix(line, ">") {
			continue
		}
		
		// Skip lines that look like quoted text
		if strings.HasPrefix(line, "On ") && strings.Contains(line, "wrote:") {
			break
		}
		
		cleanLines = append(cleanLines, line)
	}
	
	return strings.Join(cleanLines, "\n")
}

func NeedsVerification(email string) bool {
	// Common verification patterns
	verificationPatterns := []string{
		"verify",
		"verification",
		"confirm",
		"confirmation",
		"activate",
		"activation",
		"sign up",
		"signup",
		"start",
		"begin",
	}
	
	emailLower := strings.ToLower(email)
	for _, pattern := range verificationPatterns {
		if strings.Contains(emailLower, pattern) {
			return true
		}
	}
	
	return false
}