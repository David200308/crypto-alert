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

	// MetricStore for dashboard chart data
	var metricStore *store.MetricStore
	if cfg.MySQLDSN != "" {
		ms, err := store.NewMetricStore(cfg.MySQLDSN)
		if err != nil {
			log.Printf("⚠️ MetricStore disabled: %v", err)
		} else {
			metricStore = ms
			defer metricStore.Close()
			log.Println("📈 MetricStore connected — dashboard endpoints active")
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

	// Metrics routes (register before /api/logs/ catch-all)
	http.HandleFunc("/api/metrics/history", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		handleGetMetricHistory(w, r, metricStore)
	}))

	http.HandleFunc("/api/metrics", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		handleListMetrics(w, r, metricStore)
	}))

	// Log routes
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

// parseRange converts a range string (1h, 3h, 12h, 1d, 3d, 1w, 1m) to a since time.
func parseRange(rangeStr string) time.Time {
	now := time.Now().UTC()
	switch rangeStr {
	case "1h":
		return now.Add(-1 * time.Hour)
	case "3h":
		return now.Add(-3 * time.Hour)
	case "12h":
		return now.Add(-12 * time.Hour)
	case "1d":
		return now.Add(-24 * time.Hour)
	case "3d":
		return now.Add(-72 * time.Hour)
	case "1w":
		return now.Add(-7 * 24 * time.Hour)
	case "1m":
		return now.Add(-30 * 24 * time.Hour)
	default:
		return now.Add(-24 * time.Hour)
	}
}

// handleListMetrics returns all distinct (type, identifier, label, field) combinations.
// Route: GET /api/metrics
func handleListMetrics(w http.ResponseWriter, r *http.Request, ms *store.MetricStore) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	metrics, err := ms.ListMetrics()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list metrics: %v", err), http.StatusInternalServerError)
		return
	}
	if metrics == nil {
		metrics = []store.MetricInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handleGetMetricHistory returns time-series data for a given metric and time range.
// Route: GET /api/metrics/history?type=&identifier=&field=&range=1d
func handleGetMetricHistory(w http.ResponseWriter, r *http.Request, ms *store.MetricStore) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	metricType := strings.TrimSpace(q.Get("type"))
	identifier := strings.TrimSpace(q.Get("identifier"))
	field := strings.TrimSpace(q.Get("field"))
	rangeStr := strings.TrimSpace(q.Get("range"))

	if metricType == "" || identifier == "" || field == "" {
		http.Error(w, "type, identifier, and field are required", http.StatusBadRequest)
		return
	}

	since := parseRange(rangeStr)

	points, err := ms.GetMetricHistory(metricType, identifier, field, since)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get metric history: %v", err), http.StatusInternalServerError)
		return
	}
	if points == nil {
		points = []store.MetricPoint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": points})
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
