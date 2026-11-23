package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"inaly_pulse/internal/notifier"
)

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}
type Webhook struct {
	URL         string
	HTTP        Doer
	MaxAttempts int
	Sleep       func(context.Context, time.Duration) error
}

func (w *Webhook) Name() string { return "discord" }
func (w *Webhook) Notify(ctx context.Context, e notifier.Event) error {
	u, err := url.Parse(w.URL)
	if err != nil || u.Scheme != "https" || (u.Host != "discord.com" && u.Host != "discordapp.com") || !strings.HasPrefix(u.Path, "/api/webhooks/") {
		return fmt.Errorf("invalid Discord webhook URL")
	}
	body := map[string]any{"embeds": []any{map[string]any{"title": title(e.Kind), "description": description(e), "timestamp": e.OccurredAt.UTC().Format(time.RFC3339), "color": color(e.Kind)}}}
	data, _ := json.Marshal(body)
	h := w.HTTP
	if h == nil {
		h = &http.Client{Timeout: 10 * time.Second}
	}
	max := w.MaxAttempts
	if max <= 0 {
		max = 3
	}
	for attempt := 0; attempt < max; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := h.Do(req)
		if err != nil {
			if attempt+1 == max {
				return err
			}
			if err := w.sleep(ctx, time.Duration(attempt+1)*time.Second); err != nil {
				return err
			}
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return fmt.Errorf("Discord webhook returned HTTP %d", resp.StatusCode)
		}
		if attempt+1 == max {
			return fmt.Errorf("Discord webhook returned HTTP %d after retries", resp.StatusCode)
		}
		delay := retryDelay(resp.Header.Get("Retry-After"))
		if delay == 0 {
			delay = time.Duration(attempt+1) * time.Second
		}
		if err := w.sleep(ctx, delay); err != nil {
			return err
		}
	}
	return fmt.Errorf("Discord delivery attempts exhausted")
}
func (w *Webhook) sleep(ctx context.Context, d time.Duration) error {
	if w.Sleep != nil {
		return w.Sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func retryDelay(s string) time.Duration {
	if f, err := strconv.ParseFloat(s, 64); err == nil && f >= 0 {
		return time.Duration(f * float64(time.Second))
	}
	if t, err := http.ParseTime(s); err == nil && t.After(time.Now()) {
		return time.Until(t)
	}
	return 0
}
func title(k notifier.EventKind) string {
	switch k {
	case notifier.EventIncidentCreated:
		return "Incident started"
	case notifier.EventIncidentUpdated:
		return "Incident update"
	case notifier.EventIncidentResolved:
		return "Incident resolved"
	case notifier.EventMaintenanceStarted:
		return "Maintenance started"
	case notifier.EventMaintenanceEnded:
		return "Maintenance ended"
	default:
		return "Inaly Pulse event"
	}
}
func color(k notifier.EventKind) int {
	switch k {
	case notifier.EventIncidentCreated:
		return 0xE74C3C
	case notifier.EventIncidentResolved:
		return 0x2ECC71
	case notifier.EventMaintenanceStarted, notifier.EventMaintenanceEnded:
		return 0x3498DB
	default:
		return 0xF1C40F
	}
}
func description(e notifier.Event) string {
	lines := []string{}
	if e.MonitorName != "" {
		lines = append(lines, "Monitor: "+e.MonitorName)
	}
	if e.IncidentID != "" {
		lines = append(lines, "Incident: `"+e.IncidentID+"`")
	}
	if e.Summary != "" {
		lines = append(lines, e.Summary)
	}
	if e.IssueURL != "" {
		lines = append(lines, "Issue: "+e.IssueURL)
	}
	return strings.Join(lines, "\n")
}
