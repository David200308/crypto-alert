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

	// Block until the Kafka group coordinator is truly ready.
	// kafka.NewReader with a GroupID spawns a background goroutine that immediately
	// calls JoinGroup. Creating readers before the coordinator is ready floods the
	// logs with "Group Coordinator Not Available" errors from that goroutine.
	waitForGroupCoordinator(ctx, brokers)

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
	consumeWithBackoff(ctx, brokers, message.TopicTokenAlert, "notification-service-token",
		func(ctx context.Context, r *kafka.Reader) error {
			msg, err := r.FetchMessage(ctx)
			if err != nil {
				return err
			}
			var event message.TokenAlertEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("⚠️  [alerts.token] unmarshal error: %v", err)
				_ = r.CommitMessages(ctx, msg)
				return nil
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
			_ = r.CommitMessages(ctx, msg)
			return nil
		},
	)
}

// consumeDeFiAlerts reads from alerts.defi and sends DeFi alert emails.
func consumeDeFiAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender) {
	consumeWithBackoff(ctx, brokers, message.TopicDeFiAlert, "notification-service-defi",
		func(ctx context.Context, r *kafka.Reader) error {
			msg, err := r.FetchMessage(ctx)
			if err != nil {
				return err
			}
			var event message.DeFiAlertEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("⚠️  [alerts.defi] unmarshal error: %v", err)
				_ = r.CommitMessages(ctx, msg)
				return nil
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
			_ = r.CommitMessages(ctx, msg)
			return nil
		},
	)
}

// consumePredictAlerts reads from alerts.predict and sends prediction market alert emails.
func consumePredictAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender) {
	consumeWithBackoff(ctx, brokers, message.TopicPredictAlert, "notification-service-predict",
		func(ctx context.Context, r *kafka.Reader) error {
			msg, err := r.FetchMessage(ctx)
			if err != nil {
				return err
			}
			var event message.PredictMarketAlertEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("⚠️  [alerts.predict] unmarshal error: %v", err)
				_ = r.CommitMessages(ctx, msg)
				return nil
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
			_ = r.CommitMessages(ctx, msg)
			return nil
		},
	)
}

// consumeWithBackoff runs the consume loop for a topic/group, recreating the reader with
// exponential backoff whenever FetchMessage returns a persistent error. This handles transient
// broker errors (e.g. "Group Coordinator Not Available") without spinning the CPU.
func consumeWithBackoff(
	ctx context.Context,
	brokers []string,
	topic, groupID string,
	handle func(context.Context, *kafka.Reader) error,
) {
	log.Printf("🔄 [%s] consumer goroutine started, waiting for messages...", topic)

	const (
		backoffMin = 2 * time.Second
		backoffMax = 60 * time.Second
	)
	backoff := backoffMin

	for {
		if ctx.Err() != nil {
			return
		}

		r := newReader(brokers, topic, groupID)
		for {
			if err := handle(ctx, r); err != nil {
				if ctx.Err() != nil {
					r.Close()
					return
				}
				log.Printf("⚠️  [%s] read error (retrying in %v): %v", topic, backoff, err)
				r.Close()
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				// Exponential backoff, capped at backoffMax
				backoff *= 2
				if backoff > backoffMax {
					backoff = backoffMax
				}
				break // recreate the reader
			}
			backoff = backoffMin // reset on successful message
		}
	}
}

// waitForGroupCoordinator polls the Kafka group coordinator API with exponential backoff
// until it responds successfully. Using kafka.Client.FindCoordinator directly avoids
// creating a full Reader (which would itself trigger the noisy background join goroutine).
func waitForGroupCoordinator(ctx context.Context, brokers []string) {
	if len(brokers) == 0 || ctx.Err() != nil {
		return
	}
	client := &kafka.Client{
		Addr:    kafka.TCP(brokers[0]),
		Timeout: 5 * time.Second,
	}
	backoff := 1 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		resp, err := client.FindCoordinator(ctx, &kafka.FindCoordinatorRequest{
			Addr:    kafka.TCP(brokers[0]),
			Key:     "__notification_healthcheck__",
			KeyType: kafka.CoordinatorKeyTypeConsumer,
		})
		if err == nil && resp.Error == nil {
			log.Printf("✅ Kafka group coordinator is ready")
			return
		}
		reason := "unknown"
		if err != nil {
			reason = err.Error()
		} else if resp.Error != nil {
			reason = resp.Error.Error()
		}
		log.Printf("⏳ Waiting for Kafka group coordinator (%s), retrying in %v...", reason, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func newReader(brokers []string, topic, groupID string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       1,
		MaxBytes:       1e6,
		StartOffset:    kafka.FirstOffset,
		SessionTimeout: 30 * time.Second,
		MaxWait:        10 * time.Second,
		ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			log.Printf("[kafka-go][%s] ERROR: "+msg, append([]interface{}{topic}, args...)...)
		}),
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
