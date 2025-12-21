package message

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"crypto-alert/internal/core"
)

// EmailTemplateData holds data for email template rendering
type EmailTemplateData struct {
	Symbol     string
	Price      float64
	Threshold  float64
	Direction  string
	Message    string
	Timestamp  time.Time
	Confidence float64
}

// FormatAlertSubject formats the email subject for an alert
func FormatAlertSubject(symbol string, price float64, threshold float64, direction string) string {
	return fmt.Sprintf("🚨 Crypto Alert: %s %s $%g", symbol, direction, threshold)
}

// FormatAlertMessage formats the plain text message for an alert
func FormatAlertMessage(symbol string, price float64, threshold float64, direction string, timestamp time.Time, confidence float64) string {
	var directionText string
	switch direction {
	case ">=":
		directionText = "greater than or equal to"
	case ">":
		directionText = "greater than"
	case "=":
		directionText = "equal to"
	case "<=":
		directionText = "less than or equal to"
	case "<":
		directionText = "less than"
	default:
		directionText = direction
	}

	return fmt.Sprintf(`Crypto Alert Triggered!

Symbol: %s
Current Price: $%g
Threshold: $%g
Condition: Price is %s threshold
Confidence: %g%%
Timestamp: %s

This is an automated alert from your crypto price monitoring system.
`, symbol, price, threshold, directionText, confidence*100, timestamp.Format(time.RFC3339))
}

// FormatAlertHTML formats the HTML email body for an alert
func FormatAlertHTML(symbol string, price float64, threshold float64, direction string, timestamp time.Time, confidence float64) string {
	var directionText string
	var directionEmoji string
	switch direction {
	case ">=":
		directionText = "greater than or equal to"
		directionEmoji = "📈"
	case ">":
		directionText = "greater than"
		directionEmoji = "📈"
	case "=":
		directionText = "equal to"
		directionEmoji = "⚖️"
	case "<=":
		directionText = "less than or equal to"
		directionEmoji = "📉"
	case "<":
		directionText = "less than"
		directionEmoji = "📉"
	default:
		directionText = direction
		directionEmoji = "⚠️"
	}

	// Determine if price is above or below threshold for styling
	var priceColor string
	if price >= threshold {
		priceColor = "#10b981" // green
	} else {
		priceColor = "#ef4444" // red
	}

	htmlTemplate := `
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Crypto Alert</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
	<div style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px; border-radius: 10px 10px 0 0; text-align: center;">
		<h1 style="color: white; margin: 0; font-size: 28px;">🚨 Crypto Alert</h1>
	</div>
	
	<div style="background: #f9fafb; padding: 30px; border-radius: 0 0 10px 10px; border: 1px solid #e5e7eb;">
		<div style="background: white; padding: 25px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
			<h2 style="margin-top: 0; color: #1f2937; font-size: 24px;">{{.Symbol}} Alert Triggered</h2>
			
			<div style="display: flex; align-items: center; margin: 20px 0;">
				<span style="font-size: 48px; margin-right: 15px;">{{.DirectionEmoji}}</span>
				<div>
					<div style="font-size: 14px; color: #6b7280; text-transform: uppercase; letter-spacing: 1px;">Current Price</div>
					<div style="font-size: 32px; font-weight: bold; color: {{.PriceColor}};">${{.Price}}</div>
				</div>
			</div>
			
			<div style="border-top: 1px solid #e5e7eb; padding-top: 20px; margin-top: 20px;">
				<table style="width: 100%; border-collapse: collapse;">
					<tr>
						<td style="padding: 10px 0; color: #6b7280; font-weight: 500;">Symbol:</td>
						<td style="padding: 10px 0; text-align: right; font-weight: 600;">{{.Symbol}}</td>
					</tr>
					<tr>
						<td style="padding: 10px 0; color: #6b7280; font-weight: 500;">Threshold:</td>
						<td style="padding: 10px 0; text-align: right; font-weight: 600;">${{.Threshold}}</td>
					</tr>
					<tr>
						<td style="padding: 10px 0; color: #6b7280; font-weight: 500;">Condition:</td>
						<td style="padding: 10px 0; text-align: right; font-weight: 600;">Price is {{.DirectionText}} threshold</td>
					</tr>
					<tr>
						<td style="padding: 10px 0; color: #6b7280; font-weight: 500;">Confidence:</td>
						<td style="padding: 10px 0; text-align: right; font-weight: 600;">{{.Confidence}}%</td>
					</tr>
					<tr>
						<td style="padding: 10px 0; color: #6b7280; font-weight: 500;">Timestamp:</td>
						<td style="padding: 10px 0; text-align: right; font-weight: 600;">{{.Timestamp}}</td>
					</tr>
				</table>
			</div>
		</div>
		
		<div style="text-align: center; color: #6b7280; font-size: 12px; margin-top: 20px;">
			<p style="margin: 0;">This is an automated alert from your crypto price monitoring system.</p>
			<p style="margin: 5px 0 0 0;">Powered by Pyth Oracle</p>
		</div>
	</div>
</body>
</html>
`

	// Prepare template data
	data := struct {
		Symbol         string
		Price          string
		Threshold      string
		DirectionText  string
		DirectionEmoji string
		PriceColor     string
		Confidence     string
		Timestamp      string
	}{
		Symbol:         symbol,
		Price:          fmt.Sprintf("%g", price),
		Threshold:      fmt.Sprintf("%g", threshold),
		DirectionText:  directionText,
		DirectionEmoji: directionEmoji,
		PriceColor:     priceColor,
		Confidence:     fmt.Sprintf("%g", confidence*100),
		Timestamp:      timestamp.Format(time.RFC3339),
	}

	// Parse and execute template
	tmpl, err := template.New("email").Parse(htmlTemplate)
	if err != nil {
		// Fallback to simple HTML if template parsing fails
		return fmt.Sprintf(`
		<html>
		<body>
			<h1>🚨 Crypto Alert</h1>
			<h2>%s Alert Triggered</h2>
			<p><strong>Current Price:</strong> $%g</p>
			<p><strong>Threshold:</strong> $%g</p>
			<p><strong>Condition:</strong> Price is %s threshold</p>
			<p><strong>Confidence:</strong> %g%%</p>
			<p><strong>Timestamp:</strong> %s</p>
		</body>
		</html>
		`, symbol, price, threshold, directionText, confidence*100, timestamp.Format(time.RFC3339))
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		// Fallback to simple HTML if template execution fails
		return fmt.Sprintf(`
		<html>
		<body>
			<h1>🚨 Crypto Alert</h1>
			<h2>%s Alert Triggered</h2>
			<p><strong>Current Price:</strong> $%g</p>
			<p><strong>Threshold:</strong> $%g</p>
			<p><strong>Condition:</strong> Price is %s threshold</p>
			<p><strong>Confidence:</strong> %g%%</p>
			<p><strong>Timestamp:</strong> %s</p>
		</body>
		</html>
		`, symbol, price, threshold, directionText, confidence*100, timestamp.Format(time.RFC3339))
	}

	return buf.String()
}

// FormatAlertEmail formats both subject and body for an alert decision
func FormatAlertEmail(decision *core.AlertDecision) (subject, textBody, htmlBody string) {
	if decision.CurrentPrice == nil || decision.Rule == nil {
		return "", "", ""
	}

	symbol := decision.CurrentPrice.Symbol
	price := decision.CurrentPrice.Price
	threshold := decision.Rule.Threshold
	direction := string(decision.Rule.Direction)
	timestamp := decision.CurrentPrice.Timestamp
	confidence := decision.CurrentPrice.Confidence

	subject = FormatAlertSubject(symbol, price, threshold, direction)
	textBody = FormatAlertMessage(symbol, price, threshold, direction, timestamp, confidence)
	htmlBody = FormatAlertHTML(symbol, price, threshold, direction, timestamp, confidence)

	return subject, textBody, htmlBody
}
