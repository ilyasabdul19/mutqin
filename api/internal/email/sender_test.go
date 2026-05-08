// api/internal/email/sender_test.go
package email_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ilyas/mutqin-api/internal/email"
)

func TestLogSender_Records(t *testing.T) {
	s := email.NewLogSender()
	if err := s.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Code",
		Body:    "Your code is 123456",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	last := s.Last()
	if last.To != "user@example.com" {
		t.Fatalf("to: got %s", last.To)
	}
	if !strings.Contains(last.Body, "123456") {
		t.Fatalf("body: %s", last.Body)
	}
}

func TestLogSender_RejectsEmptyTo(t *testing.T) {
	s := email.NewLogSender()
	err := s.Send(context.Background(), email.Message{To: "", Subject: "x", Body: "y"})
	if err == nil {
		t.Fatal("want error on empty To")
	}
}
