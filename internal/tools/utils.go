package tools

import (
	"fmt"
	"strings"
	"time"
)

// CHARACTER_LIMIT is the default maximum response size (25,000 characters)
// This can be overridden by the character_limit parameter in tool calls
const CHARACTER_LIMIT = 25000

// truncateResponse truncates content if it exceeds the specified character limit
// Returns the original content if under limit, or truncated content with helpful message
//
//nolint:unused
func truncateResponse(content string, limit int) string {
	if limit <= 0 {
		return content // No limit specified
	}

	if len(content) <= limit {
		return content
	}

	// Reserve space for truncation message
	truncationMsg := fmt.Sprintf(
		"\n\n...[Response truncated]\n\n"+
			"Response exceeded %d character limit (was %d characters). "+
			"To see more results:\n"+
			"- Use filters to narrow results\n"+
			"- Increase character_limit parameter\n"+
			"- Request specific items by ID\n",
		limit, len(content))

	truncateAt := limit - len(truncationMsg)
	if truncateAt < 100 {
		truncateAt = 100 // Ensure at least some content
	}

	return content[:truncateAt] + truncationMsg
}

// formatTimestamp converts Unix timestamp to human-readable ISO 8601 format
// Returns empty string if timestamp is 0 or negative
//
//nolint:unused
func formatTimestamp(unixSeconds float64) string {
	if unixSeconds <= 0 {
		return ""
	}
	return time.Unix(int64(unixSeconds), 0).UTC().Format(time.RFC3339)
}

// formatDuration converts seconds to human-readable duration
//
//nolint:unused
func formatDuration(seconds float64) string {
	if seconds < 0 {
		return ""
	}

	duration := time.Duration(seconds * float64(time.Second))

	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.1fm", duration.Minutes())
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
	return fmt.Sprintf("%.1fd", duration.Hours()/24)
}

// buildMarkdownTable creates a simple markdown table from key-value pairs
//
//nolint:unused
func buildMarkdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	var md strings.Builder
	for _, row := range rows {
		if len(row) == 2 {
			md.WriteString(fmt.Sprintf("- **%s**: %s\n", row[0], row[1]))
		}
	}
	return md.String()
}
