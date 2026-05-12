// api/internal/handler/api/invite_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubInviteAcceptor struct {
	gotToken string
	gotEmail string
	gotName  string
	err      error
}

func (s *stubInviteAcceptor) AcceptInvite(_ context.Context, token, em, name string) error {
	s.gotToken = token
	s.gotEmail = em
	s.gotName = name
	return s.err
}

func TestInviteAccept_HappyPath(t *testing.T) {
	stub := &stubInviteAcceptor{}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y.com", "name": "X Y"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invite/accept", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.gotToken != "tok" || stub.gotEmail != "x@y.com" || stub.gotName != "X Y" {
		t.Fatalf("got %s %s %s", stub.gotToken, stub.gotEmail, stub.gotName)
	}
}

func TestInviteAccept_BadInvite_Returns401(t *testing.T) {
	stub := &stubInviteAcceptor{err: service.ErrInvalidInvite}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y", "name": "X"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestInviteAccept_EmailTaken_Returns409(t *testing.T) {
	stub := &stubInviteAcceptor{err: service.ErrEmailTaken}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y", "name": "X"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestInviteAccept_OtherError_Returns500(t *testing.T) {
	stub := &stubInviteAcceptor{err: errors.New("db down")}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y", "name": "X"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
}
