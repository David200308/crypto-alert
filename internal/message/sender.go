package message

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"crypto-alert/internal/core"
)

// MessageSender interface for sending alerts
type MessageSender interface {
	Send(message string) error
	SendWithSubject(subject, message string) error
	SendToEmail(toEmail, subject, message string) error
	SendAlert(toEmail string, decision *core.AlertDecision) error
}

// ResendEmailSender sends alerts via Resend API
type ResendEmailSender struct {
	apiKey    string
	fromEmail string
}

// NewResendEmailSender creates a new Resend email sender
func NewResendEmailSender(apiKey, fromEmail string) *ResendEmailSender {
	return &ResendEmailSender{
		apiKey:    apiKey,
		fromEmail: fromEmail,
	}
}

// Send sends a message via email to default recipient (not used, use SendToEmail instead)
func (r *ResendEmailSender) Send(message string) error {
	return fmt.Errorf("Send() requires recipient email, use SendToEmail() instead")
}

// SendWithSubject sends a message via email with a custom subject (not used, use SendToEmail instead)
func (r *ResendEmailSender) SendWithSubject(subject, message string) error {
	return fmt.Errorf("SendWithSubject() requires recipient email, use SendToEmail() instead")
}

// SendToEmail sends an email via Resend API to a specific recipient
func (r *ResendEmailSender) SendToEmail(toEmail, subject, message string) error {
	return r.SendToEmailWithHTML(toEmail, subject, message, "")
}

// SendToEmailWithHTML sends an email via Resend API with both text and HTML content
func (r *ResendEmailSender) SendToEmailWithHTML(toEmail, subject, textBody, htmlBody string) error {
	if r.apiKey == "" {
		return fmt.Errorf("Resend API key is not configured")
	}
	if r.fromEmail == "" {
		return fmt.Errorf("sender email is not configured")
	}
	if toEmail == "" {
		return fmt.Errorf("recipient email is required")
	}

	// Resend API endpoint
	apiURL := "https://api.resend.com/emails"

	// Prepare request payload
	payload := map[string]interface{}{
		"from":    r.fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"text":    textBody,
	}

	// Add HTML if provided
	if htmlBody != "" {
		payload["html"] = htmlBody
	} else {
		// Fallback: convert text to simple HTML
		payload["html"] = fmt.Sprintf("<p>%s</p>", strings.ReplaceAll(textBody, "\n", "<br>"))
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Resend API returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("📧 Email sent via Resend:\nTo: %s\nSubject: %s\n", toEmail, subject)
	return nil
}

// SendAlert sends an alert email using the formatted template
func (r *ResendEmailSender) SendAlert(toEmail string, decision *core.AlertDecision) error {
	subject, textBody, htmlBody := FormatAlertEmail(decision)
	return r.SendToEmailWithHTML(toEmail, subject, textBody, htmlBody)
}
