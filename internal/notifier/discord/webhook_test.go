package discord

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"inaly_pulse/internal/notifier"
)

type errorDoer struct{ err error }

func (d errorDoer) Do(*http.Request) (*http.Response, error) { return nil, d.err }

type fakeDoer struct {
	responses []*http.Response
	requests  []*http.Request
}

func (f *fakeDoer) Do(r *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, r)
	x := f.responses[0]
	f.responses = f.responses[1:]
	return x, nil
}
func response(status int, headers http.Header) *http.Response {
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(""))}
}
func TestWebhookRetriesRateLimitAndSendsEvent(t *testing.T) {
	h := &fakeDoer{responses: []*http.Response{response(429, http.Header{"Retry-After": []string{"0"}}), response(204, make(http.Header))}}
	sleeps := 0
	w := Webhook{URL: "https://discord.com/api/webhooks/123/secret", HTTP: h, Sleep: func(context.Context, time.Duration) error { sleeps++; return nil }}
	err := w.Notify(context.Background(), notifier.Event{Kind: notifier.EventIncidentCreated, IncidentID: "inc-1", OccurredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.requests) != 2 || sleeps != 1 {
		t.Fatalf("requests=%d sleeps=%d", len(h.requests), sleeps)
	}
	if h.requests[0].URL.String() != w.URL {
		t.Fatal("webhook destination changed")
	}
}
func TestWebhookRejectsUntrustedEndpoint(t *testing.T) {
	w := Webhook{URL: "https://example.org/api/webhooks/secret"}
	if err := w.Notify(context.Background(), notifier.Event{}); err == nil {
		t.Fatal("expected untrusted endpoint rejection")
	}
}

func TestWebhookDoesNotExposeURLOnTransportError(t *testing.T) {
	secretURL := "https://discord.com/api/webhooks/123/super-secret-token"
	w := Webhook{URL: secretURL, HTTP: errorDoer{err: errors.New("Post " + secretURL + ": connection refused")}, MaxAttempts: 1}
	err := w.Notify(context.Background(), notifier.Event{})
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), secretURL) || strings.Contains(err.Error(), "super-secret-token") {
		t.Fatalf("error leaked webhook URL: %v", err)
	}
}
