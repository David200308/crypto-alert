package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"crypto-alert/internal/config"
	"crypto-alert/internal/store"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logDir := cfg.LogDir
	if logDir == "" {
		logDir = "logs"
	}

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// Optional: ES client for log data (when ES is enabled)
	var esLog *store.ESClient
	if cfg.ESEnabled && len(cfg.ESAddresses) > 0 && cfg.ESIndex != "" {
		var err error
		esLog, err = store.NewESClient(cfg.ESAddresses, cfg.ESIndex)
		if err != nil {
			log.Printf("⚠️ Elasticsearch log source disabled: %v", err)
			esLog = nil
		} else {
			defer esLog.Close()
			log.Printf("📊 Log API will also read from Elasticsearch index: %s", cfg.ESIndex)
		}
	}

	// CORS middleware
	corsHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next(w, r)
		}
	}

	// Setup routes with CORS (data from files and/or Elasticsearch).
	// More-specific prefixes must be registered before the catch-all /api/logs/.
	http.HandleFunc("/api/logs/dates", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		handleGetDates(w, r, logDir, esLog)
	}))

	http.HandleFunc("/api/logs/checkpoint/", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		handleGetCheckpoint(w, r, logDir, esLog)
	}))

	http.HandleFunc("/api/logs/", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		handleGetLogs(w, r, logDir, esLog)
	}))

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8181"
	}

	log.Printf("🚀 Log API server starting on port %s", port)
	log.Printf("📁 Serving logs from: %s", logDir)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

func maskEmails(s string) string {
	return emailRegex.ReplaceAllStringFunc(s, func(email string) string {
		return "[email@address]"
	})
}

func handleGetDates(w http.ResponseWriter, r *http.Request, logDir string, esLog *store.ESClient) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dateSet := make(map[string]struct{})

	// From Elasticsearch
	if esLog != nil {
		dates, err := esLog.GetDates(r.Context())
		if err != nil {
			log.Printf("ES GetDates error: %v", err)
		} else {
			for _, d := range dates {
				dateSet[d] = struct{}{}
			}
		}
	}

	// From log files
	files, err := os.ReadDir(logDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read log directory: %v", err), http.StatusInternalServerError)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if len(name) == 12 && strings.HasSuffix(name, ".log") {
			dateStr := name[:8]
			if _, err := time.Parse("20060102", dateStr); err == nil {
				dateSet[dateStr] = struct{}{}
			}
		}
	}

	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dates)
}

// handleGetCheckpoint returns the RFC3339 timestamp of the most recent log entry for a given date.
// Route: GET /api/logs/checkpoint/{yyyyMMdd}
// Response: { "checkpoint": "<RFC3339 or empty string>" }
func handleGetCheckpoint(w http.ResponseWriter, r *http.Request, logDir string, esLog *store.ESClient) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dateStr := strings.TrimPrefix(r.URL.Path, "/api/logs/checkpoint/")
	if len(dateStr) != 8 {
		http.Error(w, "Invalid date format. Expected yyyyMMdd", http.StatusBadRequest)
		return
	}
	if _, err := time.Parse("20060102", dateStr); err != nil {
		http.Error(w, "Invalid date format. Expected yyyyMMdd", http.StatusBadRequest)
		return
	}

	var checkpoint string

	// Prefer Elasticsearch
	if esLog != nil {
		cp, err := esLog.GetCheckpoint(r.Context(), dateStr)
		if err != nil {
			log.Printf("ES GetCheckpoint error: %v", err)
		} else {
			checkpoint = cp
		}
	}

	// Fall back to log file
	if checkpoint == "" {
		logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", dateStr))
		if content, err := os.ReadFile(logFile); err == nil {
			checkpoint = store.GetCheckpointFromFile(string(content))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"checkpoint": checkpoint})
}

// handleGetLogs returns log entries for a given date.
// Route: GET /api/logs/{yyyyMMdd}[?since=<RFC3339>&q=<search>]
//   - since: when provided, returns only entries strictly after that timestamp (checkpoint diff)
//   - q:     optional message content filter
func handleGetLogs(w http.ResponseWriter, r *http.Request, logDir string, esLog *store.ESClient) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	if path == "" {
		http.Error(w, "Date parameter required", http.StatusBadRequest)
		return
	}
	if len(path) != 8 {
		http.Error(w, "Invalid date format. Expected yyyyMMdd", http.StatusBadRequest)
		return
	}
	if _, err := time.Parse("20060102", path); err != nil {
		http.Error(w, "Invalid date format. Expected yyyyMMdd", http.StatusBadRequest)
		return
	}

	since := strings.TrimSpace(r.URL.Query().Get("since")) // incremental: only return logs after this checkpoint
	searchQ := strings.TrimSpace(r.URL.Query().Get("q"))   // optional message content filter

	var entries []store.LogEntry

	// Prefer Elasticsearch when available
	if esLog != nil {
		var (
			ents []store.LogEntry
			err  error
		)
		if since != "" {
			ents, err = esLog.GetLogsSince(r.Context(), path, since, searchQ)
		} else {
			ents, err = esLog.GetLogsForDate(r.Context(), path, searchQ)
		}
		if err != nil {
			log.Printf("ES GetLogs error: %v", err)
		} else if len(ents) > 0 {
			entries = ents
		}
	}

	// Fall back to log file when no ES data
	if len(entries) == 0 {
		logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", path))
		if content, err := os.ReadFile(logFile); err == nil {
			if since != "" {
				entries = store.GetLogsFromFileSince(string(content), since, searchQ)
			} else {
				entries = store.GetLogsFromFile(string(content), searchQ)
			}
		}
	}

	// Mask emails in message for response
	for i := range entries {
		entries[i].Message = maskEmails(entries[i].Message)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": entries,
	})
}
