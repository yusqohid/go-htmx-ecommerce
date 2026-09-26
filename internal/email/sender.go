package email

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// Message encapsulates the content of an email to be dispatched.
type Message struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}

// Sender defines the abstraction for sending outgoing email messages.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPConfig holds connection and authentication details for an SMTP server.
type SMTPConfig struct {
	Host      string
	Port      string
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// SMTPSender implements Sender using Go's standard net/smtp library.
type SMTPSender struct {
	cfg SMTPConfig
}

// NewSMTPSender constructs a new SMTPSender.
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	if cfg.FromEmail == "" {
		cfg.FromEmail = "no-reply@sellora.local"
	}
	if cfg.FromName == "" {
		cfg.FromName = "Sellora"
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	return &SMTPSender{cfg: cfg}
}

// Send dispatches the email via SMTP with proper MIME headers (multipart/alternative).
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)

	boundary := fmt.Sprintf("boundary_%d", time.Now().UnixNano())

	var body strings.Builder
	body.WriteString(fmt.Sprintf("From: %s <%s>\r\n", s.cfg.FromName, s.cfg.FromEmail))
	body.WriteString(fmt.Sprintf("To: %s\r\n", msg.To))
	body.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	body.WriteString("\r\n")

	// Plain text part
	if msg.TextBody != "" {
		body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		body.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		body.WriteString("Content-Transfer-Encoding: 7bit\r\n")
		body.WriteString("\r\n")
		body.WriteString(msg.TextBody)
		body.WriteString("\r\n\r\n")
	}

	// HTML part
	if msg.HTMLBody != "" {
		body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		body.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		body.WriteString("Content-Transfer-Encoding: 7bit\r\n")
		body.WriteString("\r\n")
		body.WriteString(msg.HTMLBody)
		body.WriteString("\r\n\r\n")
	}

	body.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	var auth smtp.Auth
	if s.cfg.Username != "" && s.cfg.Password != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	to := []string{msg.To}
	return smtp.SendMail(addr, auth, s.cfg.FromEmail, to, []byte(body.String()))
}

// LogSender is a development fallback that logs outgoing emails to the console.
type LogSender struct {
	fromEmail string
	fromName  string
}

// NewLogSender creates a new LogSender for local development and debugging.
func NewLogSender(fromEmail, fromName string) *LogSender {
	if fromEmail == "" {
		fromEmail = "no-reply@sellora.local"
	}
	if fromName == "" {
		fromName = "Sellora"
	}
	return &LogSender{fromEmail: fromEmail, fromName: fromName}
}

// Send logs the email details instead of transmitting across the network.
func (l *LogSender) Send(ctx context.Context, msg Message) error {
	log.Printf("[Email:LogSender] From: %s <%s> | To: %s | Subject: %s\nText:\n%s\n",
		l.fromName, l.fromEmail, msg.To, msg.Subject, msg.TextBody)
	return nil
}

// MockSender is an in-memory sender for unit testing.
type MockSender struct {
	mu       sync.Mutex
	messages []Message
	sendErr  error
}

// NewMockSender constructs a new MockSender.
func NewMockSender() *MockSender {
	return &MockSender{messages: make([]Message, 0)}
}

// SetError injects an intentional error for testing failure branches.
func (m *MockSender) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

// Send records the message in memory.
func (m *MockSender) Send(ctx context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.messages = append(m.messages, msg)
	return nil
}

// Messages returns a copy of all recorded messages.
func (m *MockSender) Messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]Message, len(m.messages))
	copy(copied, m.messages)
	return copied
}

// Reset clears recorded messages and resets errors.
func (m *MockSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = m.messages[:0]
	m.sendErr = nil
}
