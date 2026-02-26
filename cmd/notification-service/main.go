package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"crypto-alert/internal/core"
	"crypto-alert/internal/data/price"
	"crypto-alert/internal/message"

	kafka "github.com/segmentio/kafka-go"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	brokers := envSlice("KAFKA_BROKERS", "localhost:9092")
	resendKey := os.Getenv("RESEND_API_KEY")
	resendFrom := os.Getenv("RESEND_FROM_EMAIL")

	if resendKey == "" {
		log.Fatal("RESEND_API_KEY is required")
	}
	if resendFrom == "" {
		log.Fatal("RESEND_FROM_EMAIL is required")
	}

	resend := message.NewResendEmailSender(resendKey, resendFrom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go consumeTokenAlerts(ctx, brokers, resend)
	go consumeDeFiAlerts(ctx, brokers, resend)
	go consumePredictAlerts(ctx, brokers, resend)

	log.Printf("🔔 Notification service started. Listening on brokers: %v", brokers)
	log.Println("Press Ctrl+C to stop...")

	<-sigChan
	log.Println("🛑 Shutting down notification service...")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("✅ Shutdown complete")
}

// consumeTokenAlerts reads from alerts.token and sends price alert emails.
func consumeTokenAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender) {
	r := newReader(brokers, message.TopicTokenAlert)
	defer r.Close()

	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("⚠️  [alerts.token] read error: %v", err)
			continue
		}

		var event message.TokenAlertEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("⚠️  [alerts.token] unmarshal error: %v", err)
			continue
		}

		decision := &core.AlertDecision{
			ShouldAlert: true,
			Rule: &core.AlertRule{
				Threshold: event.Threshold,
				Direction: core.Direction(event.Direction),
			},
			CurrentPrice: &price.PriceData{
				Symbol:    event.Symbol,
				Price:     event.Price,
				Timestamp: event.Timestamp,
			},
			Message: event.Message,
		}

		if err := resend.SendAlert(event.RecipientEmail, decision); err != nil {
			log.Printf("❌ [alerts.token] failed to send email to %s: %v", event.RecipientEmail, err)
		} else {
			log.Printf("✅ [alerts.token] sent alert for %s to %s", event.Symbol, event.RecipientEmail)
		}
	}
}

// consumeDeFiAlerts reads from alerts.defi and sends DeFi alert emails.
func consumeDeFiAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender) {
	r := newReader(brokers, message.TopicDeFiAlert)
	defer r.Close()

	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("⚠️  [alerts.defi] read error: %v", err)
			continue
		}

		var event message.DeFiAlertEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("⚠️  [alerts.defi] unmarshal error: %v", err)
			continue
		}

		decision := &core.DeFiAlertDecision{
			ShouldAlert: true,
			Rule: &core.DeFiAlertRule{
				Protocol:                event.Protocol,
				Category:                event.Category,
				Version:                 event.Version,
				ChainID:                 event.ChainID,
				MarketTokenContract:     event.MarketTokenContract,
				Field:                   event.Field,
				Threshold:               event.Threshold,
				Direction:               core.Direction(event.Direction),
				MarketTokenName:         event.MarketTokenName,
				MarketTokenPair:         event.MarketTokenPair,
				VaultName:               event.VaultName,
				BorrowTokenContract:     event.BorrowTokenContract,
				CollateralTokenContract: event.CollateralTokenContract,
				OracleAddress:           event.OracleAddress,
				IRMAddress:              event.IRMAddress,
				LLTV:                    event.LLTV,
				MarketContractAddress:   event.MarketContractAddress,
				VaultTokenAddress:       event.VaultTokenAddress,
				DepositTokenContract:    event.DepositTokenContract,
			},
			CurrentValue: event.CurrentValue,
			ChainName:    event.ChainName,
			Message:      event.Message,
		}

		if err := resend.SendDeFiAlert(event.RecipientEmail, decision); err != nil {
			log.Printf("❌ [alerts.defi] failed to send email to %s: %v", event.RecipientEmail, err)
		} else {
			log.Printf("✅ [alerts.defi] sent alert for %s %s to %s", event.Protocol, event.Field, event.RecipientEmail)
		}
	}
}

// consumePredictAlerts reads from alerts.predict and sends prediction market alert emails.
func consumePredictAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender) {
	r := newReader(brokers, message.TopicPredictAlert)
	defer r.Close()

	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("⚠️  [alerts.predict] read error: %v", err)
			continue
		}

		var event message.PredictMarketAlertEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("⚠️  [alerts.predict] unmarshal error: %v", err)
			continue
		}

		decision := &core.PredictMarketAlertDecision{
			ShouldAlert: true,
			Rule: &core.PredictMarketAlertRule{
				PredictMarket: event.PredictMarket,
				TokenID:       event.TokenID,
				Field:         event.Field,
				Threshold:     event.Threshold,
				Direction:     core.Direction(event.Direction),
				Question:      event.Question,
				Outcome:       event.Outcome,
				QuestionID:    event.QuestionID,
				ConditionID:   event.ConditionID,
				NegRisk:       event.NegRisk,
			},
			CurrentMidpoint:  event.CurrentMidpoint,
			CurrentBuyPrice:  event.CurrentBuyPrice,
			CurrentSellPrice: event.CurrentSellPrice,
			Message:          event.Message,
		}

		if err := resend.SendPredictMarketAlert(event.RecipientEmail, decision); err != nil {
			log.Printf("❌ [alerts.predict] failed to send email to %s: %v", event.RecipientEmail, err)
		} else {
			log.Printf("✅ [alerts.predict] sent alert for %s to %s", event.Question, event.RecipientEmail)
		}
	}
}

func newReader(brokers []string, topic string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  "notification-service",
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 1e6,
	})
}

func envSlice(key, defaultVal string) []string {
	v := os.Getenv(key)
	if v == "" {
		v = defaultVal
	}
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
