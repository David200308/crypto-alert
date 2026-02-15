package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
)

// ESConfig holds Elasticsearch connection settings for log shipping.
type ESConfig struct {
	Enabled   bool
	Addresses []string
	Index     string
}

// logDoc is the document we index per log line.
type logDoc struct {
	Timestamp string `json:"@timestamp"`
	Message   string `json:"message"`
}

// esWriter implements io.Writer and sends log lines to Elasticsearch asynchronously.
type esWriter struct {
	client *elasticsearch.Client
	index  string
	ch     chan []byte
	done   chan struct{}
	wg     sync.WaitGroup
}

// newESWriter creates an ES writer and starts the background indexer. Call Close() when done.
func newESWriter(cfg *ESConfig) (*esWriter, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Addresses,
	})
	if err != nil {
		return nil, err
	}

	w := &esWriter{
		client: client,
		index:  cfg.Index,
		ch:     make(chan []byte, 1024),
		done:   make(chan struct{}),
	}
	w.wg.Add(1)
	go w.run()
	return w, nil
}

func (w *esWriter) run() {
	defer w.wg.Done()
	ctx := context.Background()
	for {
		select {
		case <-w.done:
			return
		case p, ok := <-w.ch:
			if !ok {
				return
			}
			msg := strings.TrimSuffix(string(p), "\n")
			if msg == "" {
				continue
			}
			doc := logDoc{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Message:   msg,
			}
			body, _ := json.Marshal(doc)
			req := esapi.IndexRequest{
				Index:   w.index,
				Body:    bytes.NewReader(body),
				Refresh: "false",
			}
			res, err := req.Do(ctx, w.client)
			if err == nil && res != nil && res.Body != nil {
				_ = res.Body.Close()
			}
		}
	}
}

// Write implements io.Writer. It copies the payload and sends it to the indexer goroutine (non-blocking if buffer not full).
func (w *esWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return 0, nil
	}
	// Copy so caller can reuse buffer
	cp := make([]byte, n)
	copy(cp, p)
	select {
	case w.ch <- cp:
		return n, nil
	default:
		// Buffer full, drop to avoid blocking application logs
		return n, nil
	}
}

// Close stops the indexer and releases the ES client.
func (w *esWriter) Close() error {
	close(w.done)
	close(w.ch)
	w.wg.Wait()
	if w.client != nil {
		return w.client.Close(context.Background())
	}
	return nil
}
