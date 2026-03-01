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
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	if resendKey == "" {
		log.Fatal("RESEND_API_KEY is required")
	}
	if resendFrom == "" {
		log.Fatal("RESEND_FROM_EMAIL is required")
	}

	resend := message.NewResendEmailSender(resendKey, resendFrom)

	var tg *message.TelegramSender
	if telegramToken != "" {
		tg = message.NewTelegramSender(telegramToken)
		log.Println("📨 Telegram notifications enabled")
	} else {
		log.Println("ℹ️  TELEGRAM_BOT_TOKEN not set — Telegram notifications disabled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Block until the Kafka group coordinator is truly ready.
	// kafka.NewReader with a GroupID spawns a background goroutine that immediately
	// calls JoinGroup. Creating readers before the coordinator is ready floods the
	// logs with "Group Coordinator Not Available" errors from that goroutine.
	waitForGroupCoordinator(ctx, brokers)

	// For any consumer group that has no committed offset (fresh deploy, first run,
	// or after a coordinator failure that prevented committing), explicitly commit
	// the earliest available offset so the group starts from the beginning.
	// Groups that already have a committed offset are left completely untouched —
	// no duplicate emails on normal restarts.
	initConsumerGroupOffsets(ctx, brokers, []consumerSpec{
		{"notification-service-token", message.TopicTokenAlert},
		{"notification-service-defi", message.TopicDeFiAlert},
		{"notification-service-predict", message.TopicPredictAlert},
	})

	go consumeTokenAlerts(ctx, brokers, resend, tg)
	go consumeDeFiAlerts(ctx, brokers, resend, tg)
	go consumePredictAlerts(ctx, brokers, resend, tg)

	log.Printf("🔔 Notification service started. Listening on brokers: %v", brokers)
	log.Println("Press Ctrl+C to stop...")

	<-sigChan
	log.Println("🛑 Shutting down notification service...")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("✅ Shutdown complete")
}

// consumeTokenAlerts reads from alerts.token and sends price alert notifications.
func consumeTokenAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender, tg *message.TelegramSender) {
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
					Threshold:      event.Threshold,
					Direction:      core.Direction(event.Direction),
					TelegramChatID: event.TelegramChatID,
				},
				CurrentPrice: &price.PriceData{
					Symbol:    event.Symbol,
					Price:     event.Price,
					Timestamp: event.Timestamp,
				},
				Message: event.Message,
			}
			if event.RecipientEmail != "" {
				if err := resend.SendAlert(event.RecipientEmail, decision); err != nil {
					log.Printf("❌ [alerts.token] failed to send email to %s: %v", event.RecipientEmail, err)
				} else {
					log.Printf("✅ [alerts.token] sent email alert for %s to %s", event.Symbol, event.RecipientEmail)
				}
			}
			if tg != nil && event.TelegramChatID != "" {
				if err := tg.SendAlert(event.TelegramChatID, decision); err != nil {
					log.Printf("❌ [alerts.token] failed to send Telegram to chat %s: %v", event.TelegramChatID, err)
				} else {
					log.Printf("✅ [alerts.token] sent Telegram alert for %s to chat %s", event.Symbol, event.TelegramChatID)
				}
			}
			_ = r.CommitMessages(ctx, msg)
			return nil
		},
	)
}

// consumeDeFiAlerts reads from alerts.defi and sends DeFi alert notifications.
func consumeDeFiAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender, tg *message.TelegramSender) {
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
					TelegramChatID:          event.TelegramChatID,
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
			if event.RecipientEmail != "" {
				if err := resend.SendDeFiAlert(event.RecipientEmail, decision); err != nil {
					log.Printf("❌ [alerts.defi] failed to send email to %s: %v", event.RecipientEmail, err)
				} else {
					log.Printf("✅ [alerts.defi] sent email alert for %s %s to %s", event.Protocol, event.Field, event.RecipientEmail)
				}
			}
			if tg != nil && event.TelegramChatID != "" {
				if err := tg.SendDeFiAlert(event.TelegramChatID, decision); err != nil {
					log.Printf("❌ [alerts.defi] failed to send Telegram to chat %s: %v", event.TelegramChatID, err)
				} else {
					log.Printf("✅ [alerts.defi] sent Telegram alert for %s %s to chat %s", event.Protocol, event.Field, event.TelegramChatID)
				}
			}
			_ = r.CommitMessages(ctx, msg)
			return nil
		},
	)
}

// consumePredictAlerts reads from alerts.predict and sends prediction market alert notifications.
func consumePredictAlerts(ctx context.Context, brokers []string, resend *message.ResendEmailSender, tg *message.TelegramSender) {
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
					PredictMarket:  event.PredictMarket,
					TokenID:        event.TokenID,
					Field:          event.Field,
					Threshold:      event.Threshold,
					Direction:      core.Direction(event.Direction),
					TelegramChatID: event.TelegramChatID,
					Question:       event.Question,
					Outcome:        event.Outcome,
					QuestionID:     event.QuestionID,
					ConditionID:    event.ConditionID,
					NegRisk:        event.NegRisk,
				},
				CurrentMidpoint:  event.CurrentMidpoint,
				CurrentBuyPrice:  event.CurrentBuyPrice,
				CurrentSellPrice: event.CurrentSellPrice,
				Message:          event.Message,
			}
			if event.RecipientEmail != "" {
				if err := resend.SendPredictMarketAlert(event.RecipientEmail, decision); err != nil {
					log.Printf("❌ [alerts.predict] failed to send email to %s: %v", event.RecipientEmail, err)
				} else {
					log.Printf("✅ [alerts.predict] sent email alert for %s to %s", event.Question, event.RecipientEmail)
				}
			}
			if tg != nil && event.TelegramChatID != "" {
				if err := tg.SendPredictMarketAlert(event.TelegramChatID, decision); err != nil {
					log.Printf("❌ [alerts.predict] failed to send Telegram to chat %s: %v", event.TelegramChatID, err)
				} else {
					log.Printf("✅ [alerts.predict] sent Telegram alert for %s to chat %s", event.Question, event.TelegramChatID)
				}
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

type consumerSpec struct {
	groupID string
	topic   string
}

// initConsumerGroupOffsets ensures every consumer group starts from the earliest
// available message when no committed offset exists. On normal restarts the group
// already has a committed offset, so this function is a no-op and duplicate emails
// are never sent.
func initConsumerGroupOffsets(ctx context.Context, brokers []string, specs []consumerSpec) {
	if len(brokers) == 0 {
		return
	}
	client := &kafka.Client{
		Addr:    kafka.TCP(brokers[0]),
		Timeout: 10 * time.Second,
	}
	for _, spec := range specs {
		// Check whether the group already has a committed offset for partition 0.
		fetchResp, err := client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{
			GroupID: spec.groupID,
			Topics:  map[string][]int{spec.topic: {0}},
		})
		if err != nil {
			log.Printf("⚠️  [%s] offset check failed: %v", spec.groupID, err)
			continue
		}
		partitions := fetchResp.Topics[spec.topic]
		if len(partitions) == 0 {
			continue
		}
		p := partitions[0]
		if p.Error != nil || p.CommittedOffset >= 0 {
			// Already has a valid committed offset — leave it alone.
			if p.CommittedOffset >= 0 {
				log.Printf("📌 [%s/%s] committed offset=%d, resuming from there", spec.groupID, spec.topic, p.CommittedOffset)
			}
			continue
		}

		// No committed offset: dial the partition leader and read the earliest offset.
		conn, err := kafka.DialLeader(ctx, "tcp", brokers[0], spec.topic, 0)
		if err != nil {
			log.Printf("⚠️  [%s] dial leader error: %v", spec.groupID, err)
			continue
		}
		first, _, err := conn.ReadOffsets()
		conn.Close()
		if err != nil {
			log.Printf("⚠️  [%s] read offsets error: %v", spec.groupID, err)
			continue
		}

		// Commit the earliest offset so kafka-go starts consuming from there.
		if _, err = client.OffsetCommit(ctx, &kafka.OffsetCommitRequest{
			GroupID:      spec.groupID,
			GenerationID: -1, // -1 = standalone commit outside an active group session
			Topics: map[string][]kafka.OffsetCommit{
				spec.topic: {{Partition: 0, Offset: first}},
			},
		}); err != nil {
			log.Printf("⚠️  [%s] offset init failed: %v", spec.groupID, err)
			continue
		}
		log.Printf("📌 [%s/%s] no prior offset found, initialized to %d (earliest)", spec.groupID, spec.topic, first)
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
