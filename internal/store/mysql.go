package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"crypto-alert/internal/config"
	"crypto-alert/internal/core"

	_ "github.com/go-sql-driver/mysql"
)

const (
	tokenTable         = "alert_rule_token_config"
	defiTable          = "alert_rule_defi_config"
	predictMarketTable = "alert_rule_predict_market_config"
)

// LoadAlertRulesFromMySQL loads token and DeFi alert rules from the web3 database.
// Tables: alert_rule_token_config, alert_rule_defi_config.
// frequency and params columns are stored as JSON (MySQL JSON type is returned as []byte).
func LoadAlertRulesFromMySQL(dsn string) ([]*core.AlertRule, []*core.DeFiAlertRule, error) {
	if dsn == "" {
		return nil, nil, fmt.Errorf("MySQL DSN is required when ALERT_RULES_SOURCE=mysql")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, nil, fmt.Errorf("mysql ping: %w", err)
	}

	priceRules, err := loadTokenRules(db)
	if err != nil {
		return nil, nil, fmt.Errorf("load token rules: %w", err)
	}

	defiRules, err := loadDeFiRules(db)
	if err != nil {
		return nil, nil, fmt.Errorf("load defi rules: %w", err)
	}

	return priceRules, defiRules, nil
}

// LoadPredictMarketRulesFromMySQL loads prediction market alert rules from the web3 database.
func LoadPredictMarketRulesFromMySQL(dsn string) ([]*core.PredictMarketAlertRule, error) {
	if dsn == "" {
		return nil, fmt.Errorf("MySQL DSN is required when ALERT_RULES_SOURCE=mysql")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}

	return loadPredictMarketRules(db)
}

func loadPredictMarketRules(db *sql.DB) ([]*core.PredictMarketAlertRule, error) {
	query := `SELECT id, predict_market, params, field, threshold, direction, enabled, frequency, COALESCE(recipient_email, ''), COALESCE(telegram_chat_id, '') FROM ` + predictMarketTable
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*core.PredictMarketAlertRule
	for rows.Next() {
		var id int64
		var predictMarket, field, direction, recipientEmail, telegramChatID string
		var threshold float64
		var enabled bool
		var paramsJSON, frequencyJSON []byte

		if err := rows.Scan(&id, &predictMarket, &paramsJSON, &field, &threshold, &direction, &enabled, &frequencyJSON, &recipientEmail, &telegramChatID); err != nil {
			return nil, err
		}

		var params config.PredictMarketAlertRuleParams
		if len(paramsJSON) > 0 {
			if err := json.Unmarshal(paramsJSON, &params); err != nil {
				return nil, fmt.Errorf("predict market rule id %d: invalid params JSON: %w", id, err)
			}
		}

		rc := config.PredictMarketAlertRuleConfig{
			PredictMarket:  predictMarket,
			Params:         params,
			Field:          field,
			Threshold:      threshold,
			Direction:      direction,
			Enabled:        enabled,
			RecipientEmail: recipientEmail,
			TelegramChatID: telegramChatID,
		}
		if len(frequencyJSON) > 0 {
			var freq config.FrequencyConfig
			if err := json.Unmarshal(frequencyJSON, &freq); err != nil {
				return nil, fmt.Errorf("predict market rule id %d: invalid frequency JSON: %w", id, err)
			}
			rc.Frequency = &freq
		}

		rule, err := config.ParsePredictMarketRule(rc)
		if err != nil {
			return nil, fmt.Errorf("predict market rule id %d: %w", id, err)
		}
		rule.ID = id
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func loadTokenRules(db *sql.DB) ([]*core.AlertRule, error) {
	query := `SELECT id, symbol, price_feed_id, threshold, direction, enabled, frequency, COALESCE(recipient_email, ''), COALESCE(telegram_chat_id, '') FROM ` + tokenTable
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*core.AlertRule
	for rows.Next() {
		var id int64
		var symbol, priceFeedID, direction, recipientEmail, telegramChatID string
		var threshold float64
		var enabled bool
		var frequencyJSON []byte

		if err := rows.Scan(&id, &symbol, &priceFeedID, &threshold, &direction, &enabled, &frequencyJSON, &recipientEmail, &telegramChatID); err != nil {
			return nil, err
		}

		rc := config.AlertRuleConfig{
			Symbol:         symbol,
			PriceFeedID:    priceFeedID,
			Threshold:      threshold,
			Direction:      direction,
			Enabled:        enabled,
			RecipientEmail: recipientEmail,
			TelegramChatID: telegramChatID,
		}
		if len(frequencyJSON) > 0 {
			var freq config.FrequencyConfig
			if err := json.Unmarshal(frequencyJSON, &freq); err != nil {
				return nil, fmt.Errorf("token rule id %d: invalid frequency JSON: %w", id, err)
			}
			rc.Frequency = &freq
		}

		rule, err := config.ParsePriceRule(rc)
		if err != nil {
			return nil, fmt.Errorf("token rule id %d: %w", id, err)
		}
		rule.ID = id
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func loadDeFiRules(db *sql.DB) ([]*core.DeFiAlertRule, error) {
	query := `SELECT id, protocol, version, chain_id, params, field, threshold, direction, enabled, frequency, COALESCE(recipient_email, ''), COALESCE(telegram_chat_id, '') FROM ` + defiTable
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*core.DeFiAlertRule
	for rows.Next() {
		var id int64
		var protocol, version, chainID, field, direction, recipientEmail, telegramChatID string
		var threshold float64
		var enabled bool
		var paramsJSON, frequencyJSON []byte

		if err := rows.Scan(&id, &protocol, &version, &chainID, &paramsJSON, &field, &threshold, &direction, &enabled, &frequencyJSON, &recipientEmail, &telegramChatID); err != nil {
			return nil, err
		}

		var params config.DeFiAlertRuleParams
		if len(paramsJSON) > 0 {
			if err := json.Unmarshal(paramsJSON, &params); err != nil {
				return nil, fmt.Errorf("defi rule id %d: invalid params JSON: %w", id, err)
			}
		}

		// Optional category (for morpho/kamino) can be stored inside params JSON
		category := ""
		if len(paramsJSON) > 0 {
			var m map[string]interface{}
			if err := json.Unmarshal(paramsJSON, &m); err == nil {
				if c, ok := m["category"].(string); ok {
					category = c
				}
			}
		}

		rc := config.DeFiAlertRuleConfig{
			Protocol:       protocol,
			Category:       category,
			Version:        version,
			ChainID:        chainID,
			Field:          field,
			Threshold:      threshold,
			Direction:      direction,
			Enabled:        enabled,
			RecipientEmail: recipientEmail,
			TelegramChatID: telegramChatID,
			Params:         params,
		}
		if len(frequencyJSON) > 0 {
			var freq config.FrequencyConfig
			if err := json.Unmarshal(frequencyJSON, &freq); err != nil {
				return nil, fmt.Errorf("defi rule id %d: invalid frequency JSON: %w", id, err)
			}
			rc.Frequency = &freq
		}

		rule, err := config.ParseDeFiRule(rc)
		if err != nil {
			return nil, fmt.Errorf("defi rule id %d: %w", id, err)
		}
		rule.ID = id
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}
