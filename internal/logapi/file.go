package logapi

import (
	"strings"
	"time"
)

// Log line prefix format from Go's log.LstdFlags: "2006/01/02 15:04:05 "
const logTimePrefixLen = 19

var logTimeLayout = "2006/01/02 15:04:05"

// GetLogsFromFile parses file content (one log line per line), optionally filters by after (RFC3339) and search substring.
// Returns entries with TS from parsed prefix (or empty TS if unparseable) and nextCursor = last entry's TS.
func GetLogsFromFile(content string, after, searchQ string) ([]LogEntry, string) {
	searchLower := strings.ToLower(strings.TrimSpace(searchQ))
	afterTime := time.Time{}
	if after != "" {
		if t, err := time.Parse(time.RFC3339, after); err == nil {
			afterTime = t.UTC()
		}
	}

	var entries []LogEntry
	var nextCursor string
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
		tsStr := ""
		if !ts.IsZero() {
			tsStr = ts.Format(time.RFC3339Nano)
			if !afterTime.IsZero() && !ts.After(afterTime) {
				continue
			}
		}
		if searchLower != "" && !strings.Contains(strings.ToLower(line), searchLower) {
			continue
		}
		entries = append(entries, LogEntry{Message: line, TS: tsStr})
		nextCursor = tsStr
	}
	return entries, nextCursor
}
