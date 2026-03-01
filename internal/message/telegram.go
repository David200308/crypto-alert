package message

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"crypto-alert/internal/core"
)

// TelegramSender sends alert notifications via the Telegram Bot API.
type TelegramSender struct {
	botToken string
	client   *http.Client
}

func NewTelegramSender(botToken string) *TelegramSender {
	return &TelegramSender{
		botToken: botToken,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// sendMessage posts an HTML-formatted message to a Telegram chat.
func (t *TelegramSender) sendMessage(chatID, text string) error {
	if t.botToken == "" {
		return fmt.Errorf("telegram bot token is not configured")
	}
	if chatID == "" {
		return fmt.Errorf("telegram chat ID is required")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("📨 Telegram message sent to chat %s", chatID)
	return nil
}

// SendAlert sends a token price alert to the specified Telegram chat.
func (t *TelegramSender) SendAlert(chatID string, decision *core.AlertDecision) error {
	if chatID == "" || decision == nil || decision.Rule == nil || decision.CurrentPrice == nil {
		return nil
	}
	return t.sendMessage(chatID, formatTokenAlertTelegram(decision))
}

// SendDeFiAlert sends a DeFi protocol alert to the specified Telegram chat.
func (t *TelegramSender) SendDeFiAlert(chatID string, decision *core.DeFiAlertDecision) error {
	if chatID == "" || decision == nil || decision.Rule == nil {
		return nil
	}
	return t.sendMessage(chatID, formatDeFiAlertTelegram(decision))
}

// SendPredictMarketAlert sends a prediction market alert to the specified Telegram chat.
func (t *TelegramSender) SendPredictMarketAlert(chatID string, decision *core.PredictMarketAlertDecision) error {
	if chatID == "" || decision == nil || decision.Rule == nil {
		return nil
	}
	return t.sendMessage(chatID, formatPredictMarketAlertTelegram(decision))
}

func formatTokenAlertTelegram(decision *core.AlertDecision) string {
	r := decision.Rule
	p := decision.CurrentPrice
	emoji := telegramDirectionEmoji(string(r.Direction))
	return fmt.Sprintf(
		"🚨 <b>Crypto Alert Triggered</b>\n\n"+
			"%s <b>%s</b>\n\n"+
			"<b>Current Price:</b> $%g\n"+
			"<b>Threshold:</b> $%g\n"+
			"<b>Condition:</b> Price %s $%g\n"+
			"<b>Time:</b> %s",
		emoji, p.Symbol,
		p.Price,
		r.Threshold,
		r.Direction, r.Threshold,
		p.Timestamp.Format(time.RFC3339),
	)
}

func formatDeFiAlertTelegram(decision *core.DeFiAlertDecision) string {
	r := decision.Rule
	emoji := telegramDirectionEmoji(string(r.Direction))

	var valueStr, thresholdStr string
	if r.Field == "TVL" {
		formatted, approx := formatLargeNumber(decision.CurrentValue)
		if approx != "" {
			valueStr = fmt.Sprintf("%s (%s)", formatted, approx)
		} else {
			valueStr = formatted
		}
		thresholdFormatted, _ := formatLargeNumber(r.Threshold)
		thresholdStr = thresholdFormatted
	} else if r.Field == "APY" || r.Field == "UTILIZATION" {
		valueStr = fmt.Sprintf("%g%%", decision.CurrentValue)
		thresholdStr = fmt.Sprintf("%g%%", r.Threshold)
	} else {
		valueStr = fmt.Sprintf("%g", decision.CurrentValue)
		thresholdStr = fmt.Sprintf("%g", r.Threshold)
	}

	msg := fmt.Sprintf(
		"🚨 <b>DeFi Alert Triggered</b>\n\n"+
			"%s <b>%s %s</b> on %s\n",
		emoji, r.Protocol, r.Version, decision.ChainName,
	)

	if marketInfo := telegramBuildMarketInfo(r); marketInfo != "" {
		msg += fmt.Sprintf("<b>Market:</b> %s\n", marketInfo)
	}

	msg += fmt.Sprintf(
		"<b>Field:</b> %s\n"+
			"<b>Current Value:</b> %s\n"+
			"<b>Threshold:</b> %s\n"+
			"<b>Condition:</b> %s %s %s\n"+
			"<b>Time:</b> %s",
		r.Field,
		valueStr,
		thresholdStr,
		r.Field, r.Direction, thresholdStr,
		time.Now().UTC().Format(time.RFC3339),
	)
	return msg
}

func formatPredictMarketAlertTelegram(decision *core.PredictMarketAlertDecision) string {
	r := decision.Rule
	emoji := telegramDirectionEmoji(string(r.Direction))
	return fmt.Sprintf(
		"🚨 <b>Prediction Market Alert</b>\n\n"+
			"%s <b>%s</b>\n\n"+
			"<b>Question:</b> %s\n"+
			"<b>Outcome:</b> %s\n\n"+
			"<b>Midpoint:</b> %.4f\n"+
			"<b>Buy Price:</b> %.4f\n"+
			"<b>Sell Price:</b> %.4f\n"+
			"<b>Threshold:</b> %g\n"+
			"<b>Condition:</b> Midpoint %s %g\n"+
			"<b>Time:</b> %s",
		emoji, r.PredictMarket,
		r.Question,
		r.Outcome,
		decision.CurrentMidpoint,
		decision.CurrentBuyPrice,
		decision.CurrentSellPrice,
		r.Threshold,
		r.Direction, r.Threshold,
		time.Now().UTC().Format(time.RFC3339),
	)
}

// telegramBuildMarketInfo returns a human-readable market/vault identifier string.
func telegramBuildMarketInfo(r *core.DeFiAlertRule) string {
	if r.Protocol == "aave" && r.MarketTokenName != "" {
		return r.MarketTokenName
	}
	if r.Protocol == "morpho" {
		if r.Category == "market" && r.MarketTokenPair != "" {
			return fmt.Sprintf("%s (%s)", r.Category, r.MarketTokenPair)
		}
		if r.Category == "vault" && r.VaultName != "" {
			return fmt.Sprintf("%s (%s)", r.Category, r.VaultName)
		}
		if r.Category != "" {
			return r.Category
		}
	}
	return ""
}

// telegramDirectionEmoji returns a visual emoji for the given comparison direction.
func telegramDirectionEmoji(direction string) string {
	switch direction {
	case ">=", ">":
		return "📈"
	case "<=", "<":
		return "📉"
	case "=":
		return "⚖️"
	default:
		return "⚠️"
	}
}
