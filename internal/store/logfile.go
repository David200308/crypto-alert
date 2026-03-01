package store

import (
	"strings"
	"time"
)

// Log line prefix format from Go's log.LstdFlags: "2006/01/02 15:04:05 "
const logTimePrefixLen = 19

var logTimeLayout = "2006/01/02 15:04:05"

// GetLogsFromFile parses file content and returns all entries for the day.
// searchQ optionally filters by message substring (empty = no filter).
func GetLogsFromFile(content string, searchQ string) []LogEntry {
	return parseLogLines(content, time.Time{}, searchQ)
}

// GetLogsFromFileSince parses file content and returns only entries strictly after `since` (RFC3339).
// Used for incremental checkpoint-based updates.
func GetLogsFromFileSince(content, since, searchQ string) []LogEntry {
	sinceTime := time.Time{}
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = t.UTC()
		}
	}
	return parseLogLines(content, sinceTime, searchQ)
}

// GetCheckpointFromFile returns the RFC3339 timestamp of the last log line that has a parseable
// timestamp prefix. Returns empty string if none found.
func GetCheckpointFromFile(content string) string {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(strings.TrimSuffix(lines[i], "\r"))
		if len(line) >= logTimePrefixLen {
			if t, err := time.Parse(logTimeLayout, line[:logTimePrefixLen]); err == nil {
				return t.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	return ""
}

// parseLogLines is the shared implementation for GetLogsFromFile and GetLogsFromFileSince.
// When sinceTime is non-zero, only entries strictly after that time are included.
func parseLogLines(content string, sinceTime time.Time, searchQ string) []LogEntry {
	searchLower := strings.ToLower(strings.TrimSpace(searchQ))

	var entries []LogEntry
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var ts time.Time
		if len(trimmed) >= logTimePrefixLen {
			if t, err := time.Parse(logTimeLayout, trimmed[:logTimePrefixLen]); err == nil {
				ts = t.UTC()
			}
		}
		if !sinceTime.IsZero() && !ts.IsZero() && !ts.After(sinceTime) {
			continue
		}
		tsStr := ""
		if !ts.IsZero() {
			tsStr = ts.Format(time.RFC3339Nano)
		}
		if searchLower != "" && !strings.Contains(strings.ToLower(line), searchLower) {
			continue
		}
		entries = append(entries, LogEntry{Message: line, TS: tsStr})
	}
	return entries
}
