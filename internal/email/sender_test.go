package email_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/email"
)

func TestMockSender(t *testing.T) {
	ctx := context.Background()
	mock := email.NewMockSender()

	msg := email.Message{
		To:       "customer@example.com",
		Subject:  "Test Subject",
		HTMLBody: "<p>Hello</p>",
		TextBody: "Hello",
	}

	if err := mock.Send(ctx, msg); err != nil {
		t.Fatalf("expected send to succeed, got %v", err)
	}

	msgs := mock.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].To != "customer@example.com" {
		t.Errorf("got recipient %s, want customer@example.com", msgs[0].To)
	}

	// Test error injection
	injectedErr := errors.New("smtp connection failed")
	mock.SetError(injectedErr)
	if err := mock.Send(ctx, msg); !errors.Is(err, injectedErr) {
		t.Errorf("expected injected error, got %v", err)
	}

	// Test Reset
	mock.Reset()
	if len(mock.Messages()) != 0 {
		t.Errorf("expected messages to be cleared after reset")
	}
	if err := mock.Send(ctx, msg); err != nil {
		t.Errorf("expected send to succeed after reset, got %v", err)
	}
}

func TestLogSender(t *testing.T) {
	ctx := context.Background()
	logger := email.NewLogSender("", "")

	msg := email.Message{
		To:       "dev@example.com",
		Subject:  "Dev Email",
		HTMLBody: "<p>Dev Test</p>",
		TextBody: "Dev Test",
	}

	if err := logger.Send(ctx, msg); err != nil {
		t.Fatalf("LogSender should not return an error, got %v", err)
	}
}

func TestSMTPSender_Defaults(t *testing.T) {
	sender := email.NewSMTPSender(email.SMTPConfig{})
	if sender == nil {
		t.Fatal("expected non-nil SMTPSender")
	}
}
