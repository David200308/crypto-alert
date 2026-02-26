package message

import "time"

// Kafka topic names
const (
	TopicTokenAlert   = "alerts.token"
	TopicDeFiAlert    = "alerts.defi"
	TopicPredictAlert = "alerts.predict"
)

// TokenAlertEvent is the Kafka message payload for a price (token) alert.
type TokenAlertEvent struct {
	RecipientEmail string    `json:"recipient_email"`
	Symbol         string    `json:"symbol"`
	Price          float64   `json:"price"`
	Threshold      float64   `json:"threshold"`
	Direction      string    `json:"direction"`
	Timestamp      time.Time `json:"timestamp"`
	Message        string    `json:"message"`
}

// DeFiAlertEvent is the Kafka message payload for a DeFi protocol alert.
type DeFiAlertEvent struct {
	RecipientEmail string  `json:"recipient_email"`
	// Rule identity
	Protocol string `json:"protocol"`
	Category string `json:"category"`
	Version  string `json:"version"`
	ChainID  string `json:"chain_id"`
	ChainName string `json:"chain_name"`
	Field    string  `json:"field"`
	// Threshold
	Threshold    float64 `json:"threshold"`
	Direction    string  `json:"direction"`
	CurrentValue float64 `json:"current_value"`
	Message      string  `json:"message"`
	// Display names
	MarketTokenContract string `json:"market_token_contract"`
	MarketTokenName     string `json:"market_token_name"`
	MarketTokenPair     string `json:"market_token_pair"`
	VaultName           string `json:"vault_name"`
	// Morpho / Kamino fields
	BorrowTokenContract     string `json:"borrow_token_contract"`
	CollateralTokenContract string `json:"collateral_token_contract"`
	OracleAddress           string `json:"oracle_address"`
	IRMAddress              string `json:"irm_address"`
	LLTV                    string `json:"lltv"`
	MarketContractAddress   string `json:"market_contract_address"`
	VaultTokenAddress       string `json:"vault_token_address"`
	DepositTokenContract    string `json:"deposit_token_contract"`
}

// PredictMarketAlertEvent is the Kafka message payload for a prediction market alert.
type PredictMarketAlertEvent struct {
	RecipientEmail   string  `json:"recipient_email"`
	PredictMarket    string  `json:"predict_market"`
	TokenID          string  `json:"token_id"`
	Field            string  `json:"field"`
	Threshold        float64 `json:"threshold"`
	Direction        string  `json:"direction"`
	CurrentMidpoint  float64 `json:"current_midpoint"`
	CurrentBuyPrice  float64 `json:"current_buy_price"`
	CurrentSellPrice float64 `json:"current_sell_price"`
	Message          string  `json:"message"`
	// Display context
	Question    string `json:"question"`
	Outcome     string `json:"outcome"`
	QuestionID  string `json:"question_id"`
	ConditionID string `json:"condition_id"`
	NegRisk     bool   `json:"neg_risk"`
}
