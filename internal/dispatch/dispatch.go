package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"inaly_pulse/internal/app"
	"inaly_pulse/internal/githubapp"
	"inaly_pulse/internal/notifier"
	discordnotifier "inaly_pulse/internal/notifier/discord"
)

type Options struct {
	DataDir, Owner, Repo, DiscordWebhook string
	App                                  *githubapp.Client
	Settings                             app.NotificationSettings
	Now                                  time.Time
}

func Run(ctx context.Context, o Options) error {
	incidents, err := readIncidents(filepath.Join(o.DataDir, "incidents", "active.json"))
	if err != nil {
		return err
	}
	events, err := readOutbox(filepath.Join(o.DataDir, "outbox", "pending.json"))
	if err != nil {
		return err
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if o.App != nil {
		if o.Owner == "" || o.Repo == "" {
			return fmt.Errorf("owner/repository required for issue handling")
		}
		for i := range events {
			e := &events[i]
			if e.IssueDone || !strings.HasPrefix(e.Type, "incident_") {
				continue
			}
			if err := applyIssue(ctx, o, *e, &incidents); err != nil {
				e.IssueError = err.Error()
				continue
			}
			e.IssueDone = true
			e.IssueError = ""
		}
		for i := range incidents {
			inc := &incidents[i]
			if inc.IssueNumber == 0 {
				continue
			}
			issue, err := o.App.GetIssue(ctx, o.Owner, o.Repo, inc.IssueNumber)
			if err != nil && isNotFound(err) {
				if replacement, recreateErr := ensureIssue(ctx, o, inc, true, ""); recreateErr != nil {
					continue
				} else if inc.State == "resolved" && replacement != nil {
					_ = o.App.CommentIssue(ctx, o.Owner, o.Repo, replacement.Number, "This issue was recreated for an existing resolved incident.")
					if replacement.State == "open" {
						_ = o.App.CloseIssue(ctx, o.Owner, o.Repo, replacement.Number)
					}
				}
			} else if err == nil {
				inc.IssueURL = issue.HTMLURL
			}
		}
	}
	if o.Settings.Discord.Enabled && o.DiscordWebhook != "" {
		d := &discordnotifier.Webhook{URL: o.DiscordWebhook}
		for i := range events {
			e := &events[i]
			if e.Status == "delivered" || !notifyEnabled(o.Settings.Discord.NotifyOn, e.Type) {
				continue
			}
			if e.Attempts >= 20 {
				e.Status = "failed"
				continue
			}
			e.Attempts++
			e.LastAttemptAt = &now
			event := toNotifierEvent(*e)
			err := d.Notify(ctx, event)
			if err != nil {
				e.Status = "failed"
				e.LastError = err.Error()
			} else {
				e.Status = "delivered"
				e.LastError = ""
				e.DeliveredAt = &now
			}
		}
	}
	if err := writeJSON(filepath.Join(o.DataDir, "incidents", "active.json"), incidents); err != nil {
		return err
	}
	return writeJSON(filepath.Join(o.DataDir, "outbox", "pending.json"), events)
}

func applyIssue(ctx context.Context, o Options, e app.OutboxEvent, incidents *[]app.Incident) error {
	index := -1
	for i := range *incidents {
		if (*incidents)[i].ID == e.AggregateID {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("incident %s not found", e.AggregateID)
	}
	inc := &(*incidents)[index]
	createIfMissing := e.Type == "incident_created" || e.Type == "incident_updated" && inc.State == "open" || e.Type == "incident_resolved" && inc.IssueNumber > 0
	issue, err := ensureIssue(ctx, o, inc, createIfMissing, payloadString(e.Payload, "summary"))
	if err != nil {
		return err
	}
	if issue == nil {
		return nil
	}
	inc.IssueNumber = issue.Number
	inc.IssueURL = issue.HTMLURL
	e.Payload["issue_url"] = issue.HTMLURL
	switch e.Type {
	case "incident_updated":
		return o.App.CommentIssue(ctx, o.Owner, o.Repo, issue.Number, fmt.Sprintf("Incident update (%s): %s", e.CreatedAt.UTC().Format(time.RFC3339), payloadString(e.Payload, "summary")))
	case "incident_resolved":
		if err := o.App.CommentIssue(ctx, o.Owner, o.Repo, issue.Number, fmt.Sprintf("Incident resolved at %s.", payloadString(e.Payload, "resolved_at"))); err != nil {
			return err
		}
		if issue.State == "open" {
			return o.App.CloseIssue(ctx, o.Owner, o.Repo, issue.Number)
		}
	}
	return nil
}

func ensureIssue(ctx context.Context, o Options, inc *app.Incident, createIfMissing bool, summary string) (*githubapp.Issue, error) {
	var issue *githubapp.Issue
	if inc.IssueNumber > 0 {
		found, err := o.App.GetIssue(ctx, o.Owner, o.Repo, inc.IssueNumber)
		if err == nil {
			issue = &found
		} else if !isNotFound(err) {
			return nil, err
		}
	}
	if issue == nil {
		found, err := o.App.FindIncidentIssue(ctx, o.Owner, o.Repo, inc.ID)
		if err != nil {
			return nil, err
		}
		issue = found
	}
	if issue == nil && createIfMissing {
		body := fmt.Sprintf("Monitor **%s** (`%s`) entered outage at %s.\n\nIncident ID: `%s`\n\nThis issue is managed by Inaly Pulse.", inc.MonitorName, inc.MonitorID, inc.OpenedAt.UTC().Format(time.RFC3339), inc.ID)
		if summary == "" {
			summary = inc.LastError
		}
		if summary != "" {
			body += "\n\nInitial error: " + summary
		}
		// Record the previous mapping before building the replacement body so the
		// first recreated issue is clearly identified as such.
		wasRecreated := len(inc.IssueHistory) > 0
		if wasRecreated {
			body += "\n\nThis issue was recreated after its previous issue was deleted."
		}
		created, err := o.App.CreateIncidentIssue(ctx, o.Owner, o.Repo, inc.ID, "[Inaly Pulse] Outage: "+inc.MonitorName, body)
		if err != nil {
			return nil, err
		}
		issue = &created
	}
	if issue != nil && inc.IssueNumber != issue.Number {
		if inc.IssueNumber > 0 {
			replaced := time.Now().UTC()
			inc.IssueHistory = append(inc.IssueHistory, app.IssueRef{Number: inc.IssueNumber, URL: inc.IssueURL, ReplacedAt: &replaced})
		}
		inc.IssueNumber = issue.Number
		inc.IssueURL = issue.HTMLURL
	}
	return issue, nil
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "HTTP 410")
}
func notifyEnabled(settings map[string]bool, kind string) bool {
	if v, ok := settings[kind]; ok {
		return v
	}
	if strings.HasPrefix(kind, "maintenance_") {
		if v, ok := settings["maintenance"]; ok {
			return v
		}
	}
	return kind == "incident_created" || kind == "incident_updated" || kind == "incident_resolved"
}
func toNotifierEvent(e app.OutboxEvent) notifier.Event {
	n := notifier.Event{Kind: notifier.EventKind(e.Type), DedupeKey: e.ID, IncidentID: payloadString(e.Payload, "incident_id"), MonitorID: payloadString(e.Payload, "monitor_id"), MonitorName: payloadString(e.Payload, "monitor_name"), Summary: payloadString(e.Payload, "summary"), IssueURL: payloadString(e.Payload, "issue_url"), OccurredAt: e.CreatedAt}
	if s := payloadString(e.Payload, "occurred_at"); s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			n.OccurredAt = t
		}
	}
	if s := payloadString(e.Payload, "resolved_at"); s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			n.ResolvedAt = &t
		}
	}
	if strings.HasPrefix(e.Type, "maintenance_") {
		n.Summary = "Maintenance: " + payloadString(e.Payload, "maintenance_id")
	}
	return n
}
func payloadString(p map[string]any, key string) string {
	v := p[key]
	switch x := v.(type) {
	case string:
		return x
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}
func readIncidents(path string) ([]app.Incident, error) {
	var v []app.Incident
	err := read(path, &v)
	return v, err
}
func readOutbox(path string) ([]app.OutboxEvent, error) {
	var v []app.OutboxEvent
	err := read(path, &v)
	return v, err
}
func read(path string, v any) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
