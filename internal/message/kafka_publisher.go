package message

import (
	"context"
	"encoding/json"
	"fmt"

	"crypto-alert/internal/core"

	kafka "github.com/segmentio/kafka-go"
)

// KafkaAlertPublisher implements MessageSender by publishing alert events to Kafka.
// The notification-service consumes these events and delivers emails via Resend.
type KafkaAlertPublisher struct {
	writer *kafka.Writer
}

// NewKafkaAlertPublisher creates a publisher that writes to the given Kafka brokers.
func NewKafkaAlertPublisher(brokers []string) *KafkaAlertPublisher {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Balancer: &kafka.LeastBytes{},
	}
	return &KafkaAlertPublisher{writer: w}
}

// Close shuts down the underlying Kafka writer.
func (p *KafkaAlertPublisher) Close() error {
	return p.writer.Close()
}

func (p *KafkaAlertPublisher) Send(_ string) error {
	return fmt.Errorf("Send() not supported by KafkaAlertPublisher")
}

func (p *KafkaAlertPublisher) SendWithSubject(_, _ string) error {
	return fmt.Errorf("SendWithSubject() not supported by KafkaAlertPublisher")
}

func (p *KafkaAlertPublisher) SendToEmail(_, _, _ string) error {
	return fmt.Errorf("SendToEmail() not supported by KafkaAlertPublisher")
}

// SendAlert publishes a token price alert to the alerts.token Kafka topic.
func (p *KafkaAlertPublisher) SendAlert(toEmail string, decision *core.AlertDecision) error {
	event := TokenAlertEvent{
		RecipientEmail: toEmail,
		Symbol:         decision.CurrentPrice.Symbol,
		Price:          decision.CurrentPrice.Price,
		Timestamp:      decision.CurrentPrice.Timestamp,
		Threshold:      decision.Rule.Threshold,
		Direction:      string(decision.Rule.Direction),
		Message:        decision.Message,
	}
	return p.publish(TopicTokenAlert, event)
}

// SendDeFiAlert publishes a DeFi alert to the alerts.defi Kafka topic.
func (p *KafkaAlertPublisher) SendDeFiAlert(toEmail string, decision *core.DeFiAlertDecision) error {
	r := decision.Rule
	event := DeFiAlertEvent{
		RecipientEmail:          toEmail,
		Protocol:                r.Protocol,
		Category:                r.Category,
		Version:                 r.Version,
		ChainID:                 r.ChainID,
		ChainName:               decision.ChainName,
		MarketTokenContract:     r.MarketTokenContract,
		Field:                   r.Field,
		Threshold:               r.Threshold,
		Direction:               string(r.Direction),
		CurrentValue:            decision.CurrentValue,
		Message:                 decision.Message,
		MarketTokenName:         r.MarketTokenName,
		MarketTokenPair:         r.MarketTokenPair,
		VaultName:               r.VaultName,
		BorrowTokenContract:     r.BorrowTokenContract,
		CollateralTokenContract: r.CollateralTokenContract,
		OracleAddress:           r.OracleAddress,
		IRMAddress:              r.IRMAddress,
		LLTV:                    r.LLTV,
		MarketContractAddress:   r.MarketContractAddress,
		VaultTokenAddress:       r.VaultTokenAddress,
		DepositTokenContract:    r.DepositTokenContract,
	}
	return p.publish(TopicDeFiAlert, event)
}

// SendPredictMarketAlert publishes a prediction market alert to the alerts.predict Kafka topic.
func (p *KafkaAlertPublisher) SendPredictMarketAlert(toEmail string, decision *core.PredictMarketAlertDecision) error {
	r := decision.Rule
	event := PredictMarketAlertEvent{
		RecipientEmail:   toEmail,
		PredictMarket:    r.PredictMarket,
		TokenID:          r.TokenID,
		Field:            r.Field,
		Threshold:        r.Threshold,
		Direction:        string(r.Direction),
		CurrentMidpoint:  decision.CurrentMidpoint,
		CurrentBuyPrice:  decision.CurrentBuyPrice,
		CurrentSellPrice: decision.CurrentSellPrice,
		Message:          decision.Message,
		Question:         r.Question,
		Outcome:          r.Outcome,
		QuestionID:       r.QuestionID,
		ConditionID:      r.ConditionID,
		NegRisk:          r.NegRisk,
	}
	return p.publish(TopicPredictAlert, event)
}

func (p *KafkaAlertPublisher) publish(topic string, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal kafka event for topic %s: %w", topic, err)
	}
	return p.writer.WriteMessages(context.Background(), kafka.Message{
		Topic: topic,
		Value: data,
	})
}
