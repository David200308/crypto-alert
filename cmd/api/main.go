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

	// Setup routes with CORS
	http.HandleFunc("/api/logs/dates", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		handleGetDates(w, r, logDir)
	}))

	http.HandleFunc("/api/logs/", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		handleGetLogs(w, r, logDir)
	}))

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8181"
	}

	log.Printf("🚀 Log API server starting on port %s", port)
	log.Printf("📁 Serving logs from: %s", logDir)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleGetDates(w http.ResponseWriter, r *http.Request, logDir string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files, err := os.ReadDir(logDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read log directory: %v", err), http.StatusInternalServerError)
		return
	}

	dates := []string{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		// Check if file matches yyyyMMdd.log format
		if len(name) == 12 && strings.HasSuffix(name, ".log") {
			dateStr := name[:8]
			// Validate date format
			if _, err := time.Parse("20060102", dateStr); err == nil {
				dates = append(dates, dateStr)
			}
		}
	}

	// Sort dates descending (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dates)
}

func handleGetLogs(w http.ResponseWriter, r *http.Request, logDir string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract date from URL path: /api/logs/20260107
	path := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	if path == "" {
		http.Error(w, "Date parameter required", http.StatusBadRequest)
		return
	}

	// Validate date format (yyyyMMdd)
	if len(path) != 8 {
		http.Error(w, "Invalid date format. Expected yyyyMMdd", http.StatusBadRequest)
		return
	}

	// Validate date is parseable
	if _, err := time.Parse("20060102", path); err != nil {
		http.Error(w, "Invalid date format. Expected yyyyMMdd", http.StatusBadRequest)
		return
	}

	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", path))

	// Check if file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs": []string{},
		})
		return
	}

	// Read log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read log file: %v", err), http.StatusInternalServerError)
		return
	}

	// Split into lines
	lines := strings.Split(string(content), "\n")
	// Filter out empty lines and mask email addresses
	logLines := []string{}
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			// Mask email addresses
			maskedLine := emailRegex.ReplaceAllStringFunc(line, func(email string) string {
				return "[email@address]"
			})
			logLines = append(logLines, maskedLine)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logLines,
	})
}
