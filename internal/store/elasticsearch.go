package store

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
)

// ESClient queries Elasticsearch for log dates and log lines (e.g. crypto-alert-logs index).
type ESClient struct {
	client *elasticsearch.Client
	index  string
}

// NewESClient creates a client for querying logs from ES. Caller should close the client when done.
func NewESClient(addresses []string, index string) (*ESClient, error) {
	if len(addresses) == 0 || index == "" {
		return nil, nil
	}
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: addresses})
	if err != nil {
		return nil, err
	}
	return &ESClient{client: client, index: index}, nil
}

// Close releases the ES client.
func (c *ESClient) Close() error {
	if c != nil && c.client != nil {
		return c.client.Close(context.Background())
	}
	return nil
}

// GetDates returns sorted list of dates (yyyyMMdd) that have logs in ES, most recent first.
func (c *ESClient) GetDates(ctx context.Context) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	body := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			"by_day": map[string]interface{}{
				"date_histogram": map[string]interface{}{
					"field":    "@timestamp",
					"interval": "day",
					"format":   "yyyyMMdd",
				},
			},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	req := esapi.SearchRequest{
		Index: []string{c.index},
		Body:  &buf,
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, errFromESResponse(res)
	}
	var out struct {
		Aggregations struct {
			ByDay struct {
				Buckets []struct {
					KeyAsString string `json:"key_as_string"`
				} `json:"buckets"`
			} `json:"by_day"`
		} `json:"aggregations"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	dates := make([]string, 0, len(out.Aggregations.ByDay.Buckets))
	for _, b := range out.Aggregations.ByDay.Buckets {
		if b.KeyAsString != "" {
			dates = append(dates, b.KeyAsString)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates, nil
}

// LogEntry is a single log line with a parsed timestamp.
type LogEntry struct {
	Message string `json:"message"`
	TS      string `json:"ts"` // RFC3339
}

// buildQuery wraps a range query with an optional full-text search on message.
func buildQuery(tsRange map[string]interface{}, searchQ string) map[string]interface{} {
	rangeQ := map[string]interface{}{"range": map[string]interface{}{"@timestamp": tsRange}}
	if searchQ == "" {
		return rangeQ
	}
	return map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []interface{}{
				rangeQ,
				map[string]interface{}{
					"simple_query_string": map[string]interface{}{
						"query":  searchQ,
						"fields": []string{"message"},
					},
				},
			},
		},
	}
}

// fetchESLogs pages through ES results for the given query using search_after, returning all entries.
func (c *ESClient) fetchESLogs(ctx context.Context, query map[string]interface{}) ([]LogEntry, error) {
	const pageSize = 10000
	sortClause := []map[string]interface{}{{"@timestamp": map[string]string{"order": "asc"}}}

	var allEntries []LogEntry
	var searchAfter []interface{}

	for {
		body := map[string]interface{}{
			"size":    pageSize,
			"sort":    sortClause,
			"_source": []string{"message", "@timestamp"},
			"query":   query,
		}
		if len(searchAfter) > 0 {
			body["search_after"] = searchAfter
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
		res, err := esapi.SearchRequest{Index: []string{c.index}, Body: &buf}.Do(ctx, c.client)
		if err != nil {
			return nil, err
		}
		var out struct {
			Hits struct {
				Hits []struct {
					Source struct {
						Message   string `json:"message"`
						Timestamp string `json:"@timestamp"`
					} `json:"_source"`
					Sort []interface{} `json:"sort"`
				} `json:"hits"`
			} `json:"hits"`
		}
		decodeErr := json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		if res.IsError() {
			return nil, errFromESResponse(res)
		}
		if decodeErr != nil {
			return nil, decodeErr
		}

		hits := out.Hits.Hits
		if len(hits) == 0 {
			break
		}
		for _, h := range hits {
			msg := strings.TrimSpace(h.Source.Message)
			if msg != "" {
				allEntries = append(allEntries, LogEntry{Message: msg, TS: h.Source.Timestamp})
			}
		}
		if len(hits) < pageSize {
			break
		}
		searchAfter = hits[len(hits)-1].Sort
		if len(searchAfter) == 0 {
			break
		}
	}
	return allEntries, nil
}

// GetLogsForDate returns all log entries for the given date (yyyyMMdd), optionally filtered by searchQ.
// Pages through ES automatically using search_after to return the complete day's logs.
func (c *ESClient) GetLogsForDate(ctx context.Context, dateStr, searchQ string) ([]LogEntry, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return nil, err
	}
	start := t.UTC().Format(time.RFC3339)
	end := t.Add(24 * time.Hour).UTC().Format(time.RFC3339)
	return c.fetchESLogs(ctx, buildQuery(map[string]interface{}{"gte": start, "lt": end}, searchQ))
}

// GetLogsSince returns only log entries that arrived strictly after `since` (RFC3339) for the given date.
// Used for incremental checkpoint-based updates.
func (c *ESClient) GetLogsSince(ctx context.Context, dateStr, since, searchQ string) ([]LogEntry, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return nil, err
	}
	end := t.Add(24 * time.Hour).UTC().Format(time.RFC3339)
	return c.fetchESLogs(ctx, buildQuery(map[string]interface{}{"gt": since, "lt": end}, searchQ))
}

// GetCheckpoint returns the RFC3339 timestamp of the most recent log entry for the given date.
// Returns an empty string when no entries exist.
func (c *ESClient) GetCheckpoint(ctx context.Context, dateStr string) (string, error) {
	if c == nil || c.client == nil {
		return "", nil
	}
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return "", err
	}
	start := t.UTC().Format(time.RFC3339)
	end := t.Add(24 * time.Hour).UTC().Format(time.RFC3339)

	body := map[string]interface{}{
		"size":    1,
		"sort":    []map[string]interface{}{{"@timestamp": map[string]string{"order": "desc"}}},
		"_source": []string{"@timestamp"},
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{"gte": start, "lt": end},
			},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return "", err
	}
	res, err := esapi.SearchRequest{Index: []string{c.index}, Body: &buf}.Do(ctx, c.client)
	if err != nil {
		return "", err
	}
	var out struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Timestamp string `json:"@timestamp"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	decodeErr := json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	if res.IsError() {
		return "", errFromESResponse(res)
	}
	if decodeErr != nil {
		return "", decodeErr
	}
	if len(out.Hits.Hits) == 0 {
		return "", nil
	}
	return out.Hits.Hits[0].Source.Timestamp, nil
}

func errFromESResponse(res *esapi.Response) error {
	var e struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	}
	_ = json.NewDecoder(res.Body).Decode(&e)
	if e.Error.Reason != "" {
		return &esError{status: res.StatusCode, reason: e.Error.Reason}
	}
	return &esError{status: res.StatusCode, reason: res.String()}
}

type esError struct {
	status int
	reason string
}

func (e *esError) Error() string {
	return e.reason
}
