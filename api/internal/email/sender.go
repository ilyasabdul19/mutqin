// api/internal/email/sender.go
//
// Email delivery interface. Plan H ships a slog-based stub (LogSender) for
// dev + tests. A real SMTP/Resend implementation lands in a follow-up plan
// gated on RESEND_API_KEY. Until then, OTP codes appear in the api server logs.
package email

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// Message is a single email to send.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender delivers emails. Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// LogSender writes the message to slog at INFO level and records the last
// message for tests to inspect.
type LogSender struct {
	mu   sync.Mutex
	last Message
}

func NewLogSender() *LogSender { return &LogSender{} }

func (s *LogSender) Send(ctx context.Context, m Message) error {
	if m.To == "" {
		return errors.New("email: empty To")
	}
	s.mu.Lock()
	s.last = m
	s.mu.Unlock()
	slog.InfoContext(ctx, "email send (LogSender)",
		"to", m.To, "subject", m.Subject, "body", m.Body)
	return nil
}

func (s *LogSender) Last() Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}
