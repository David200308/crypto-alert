package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MetricPoint struct {
	Value      float64 `json:"value"`
	RecordedAt string  `json:"recorded_at"`
}

type MetricInfo struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
	Label      string `json:"label"`
	Field      string `json:"field"`
	Enabled    bool   `json:"enabled"`
}

type MetricStore struct {
	db *sql.DB
}

func NewMetricStore(dsn string) (*MetricStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	return &MetricStore{db: db}, nil
}

func (s *MetricStore) Close() {
	if s != nil && s.db != nil {
		s.db.Close()
	}
}

func (s *MetricStore) InsertMetricSnapshot(metricType, identifier, label, field string, value float64) error {
	if s == nil {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO metric_snapshots (type, identifier, label, field, value, recorded_at) VALUES (?, ?, ?, ?, ?, UTC_TIMESTAMP())`,
		metricType, identifier, label, field, value,
	)
	return err
}

func (s *MetricStore) GetMetricHistory(metricType, identifier, field string, since time.Time) ([]MetricPoint, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT value, recorded_at FROM metric_snapshots WHERE type=? AND identifier=? AND field=? AND recorded_at >= ? ORDER BY recorded_at ASC`,
		metricType, identifier, field, since.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []MetricPoint
	for rows.Next() {
		var value float64
		var ts []byte
		if err := rows.Scan(&value, &ts); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, string(ts))
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", string(ts))
		}
		if err != nil {
			return nil, fmt.Errorf("parse recorded_at %q: %w", string(ts), err)
		}
		points = append(points, MetricPoint{
			Value:      value,
			RecordedAt: t.UTC().Format(time.RFC3339),
		})
	}
	return points, rows.Err()
}

func (s *MetricStore) ListMetrics() ([]MetricInfo, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`
		SELECT DISTINCT ms.type, ms.identifier, ms.label, ms.field,
			CASE ms.type
				WHEN 'token' THEN COALESCE(
					(SELECT MAX(CAST(enabled AS UNSIGNED))
					 FROM alert_rule_token_config WHERE symbol = ms.identifier), 1)
				WHEN 'predict' THEN COALESCE(
					(SELECT MAX(CAST(enabled AS UNSIGNED))
					 FROM alert_rule_predict_market_config
					 WHERE JSON_UNQUOTE(JSON_EXTRACT(params, '$.token_id')) = ms.identifier), 1)
				WHEN 'defi' THEN COALESCE(
					(SELECT MAX(CAST(enabled AS UNSIGNED))
					 FROM alert_rule_defi_config
					 WHERE field = ms.field
					 AND CONCAT(protocol, '-', version, '-', chain_id, '-',
						 COALESCE(
							 JSON_UNQUOTE(JSON_EXTRACT(params, '$.vault_token_address')),
							 JSON_UNQUOTE(JSON_EXTRACT(params, '$.ledger_address')),
							 JSON_UNQUOTE(JSON_EXTRACT(params, '$.market_id')),
							 JSON_UNQUOTE(JSON_EXTRACT(params, '$.market_token_contract')),
							 ''
						 )
					 ) = ms.identifier), 1)
				ELSE 1
			END AS enabled
		FROM metric_snapshots ms
		ORDER BY ms.type, ms.identifier, ms.field`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []MetricInfo
	for rows.Next() {
		var m MetricInfo
		var enabled int
		if err := rows.Scan(&m.Type, &m.Identifier, &m.Label, &m.Field, &enabled); err != nil {
			return nil, err
		}
		m.Enabled = enabled != 0
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}
