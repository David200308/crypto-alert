package logapi

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
		return nil, errFromResponse(res)
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

// LogEntry is a single log line with a timestamp for cursor-based pagination.
type LogEntry struct {
	Message string `json:"message"`
	TS      string `json:"ts"` // RFC3339
}

// GetLogsForDate returns log entries for the given date (yyyyMMdd), optionally after a cursor and filtered by search.
// after is RFC3339; empty means from start of day. searchQ filters by message content (empty = no filter).
// nextCursor is the TS of the last entry (for the next request's after).
func (c *ESClient) GetLogsForDate(ctx context.Context, dateStr, after, searchQ string) ([]LogEntry, string, error) {
	if c == nil || c.client == nil {
		return nil, "", nil
	}
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return nil, "", err
	}
	start := t.UTC().Format(time.RFC3339)
	end := t.Add(24 * time.Hour).UTC().Format(time.RFC3339)

	tsRange := map[string]string{"lt": end}
	if after != "" {
		tsRange["gt"] = after
	} else {
		tsRange["gte"] = start
	}
	rangeQ := map[string]interface{}{"range": map[string]interface{}{"@timestamp": tsRange}}

	var query map[string]interface{}
	if searchQ != "" {
		query = map[string]interface{}{
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
	} else {
		query = rangeQ
	}

	body := map[string]interface{}{
		"size":    10000,
		"sort":    []map[string]interface{}{{"@timestamp": map[string]string{"order": "asc"}}},
		"_source": []string{"message", "@timestamp"},
		"query":   query,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, "", err
	}
	req := esapi.SearchRequest{
		Index: []string{c.index},
		Body:  &buf,
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, "", errFromResponse(res)
	}
	var out struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Message   string `json:"message"`
					Timestamp string `json:"@timestamp"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, "", err
	}
	entries := make([]LogEntry, 0, len(out.Hits.Hits))
	var nextCursor string
	for _, h := range out.Hits.Hits {
		msg := strings.TrimSpace(h.Source.Message)
		if msg == "" {
			continue
		}
		ts := h.Source.Timestamp
		entries = append(entries, LogEntry{Message: msg, TS: ts})
		nextCursor = ts
	}
	return entries, nextCursor, nil
}

func errFromResponse(res *esapi.Response) error {
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
