package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Options struct {
	ConfigPath string
	DataDir    string
	Run        bool
	Force      bool
	Now        time.Time
	Check      CheckFunc
}
type CheckFunc func(context.Context, MonitorConfig, time.Duration) CheckResult
type CheckResult struct {
	MonitorID  string        `json:"monitor_id"`
	Type       string        `json:"type"`
	Status     string        `json:"status"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
	Details    any           `json:"details,omitempty"`
}
type MonitorState struct {
	State         string     `json:"state"`
	FailureCount  int        `json:"failure_count"`
	RecoveryCount int        `json:"recovery_count"`
	LastRun       *time.Time `json:"last_run,omitempty"`
}
type Incident struct {
	ID           string     `json:"id"`
	MonitorID    string     `json:"monitor_id"`
	MonitorName  string     `json:"monitor_name"`
	State        string     `json:"state"`
	OpenedAt     time.Time  `json:"opened_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
	IssueNumber  int        `json:"issue_number,omitempty"`
	IssueURL     string     `json:"issue_url,omitempty"`
	IssueHistory []IssueRef `json:"issue_history,omitempty"`
	NeedsIssue   bool       `json:"needs_issue,omitempty"`
}
type IssueRef struct {
	Number     int        `json:"number"`
	URL        string     `json:"url"`
	ReplacedAt *time.Time `json:"replaced_at,omitempty"`
}
type OutboxEvent struct {
	ID            string         `json:"event_id"`
	Type          string         `json:"event_type"`
	AggregateID   string         `json:"aggregate_id"`
	CreatedAt     time.Time      `json:"created_at"`
	Payload       map[string]any `json:"payload"`
	Status        string         `json:"delivery_status"`
	Attempts      int            `json:"attempt_count"`
	LastAttemptAt *time.Time     `json:"last_attempt_at,omitempty"`
	LastError     string         `json:"last_error,omitempty"`
	DeliveredAt   *time.Time     `json:"delivered_at,omitempty"`
	IssueDone     bool           `json:"issue_done,omitempty"`
	IssueError    string         `json:"issue_error,omitempty"`
}
type Report struct {
	SchemaVersion int           `json:"schema_version"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Checks        []CheckResult `json:"checks"`
}
type Output struct {
	Mode       string        `json:"mode"`
	ConfigPath string        `json:"config_path"`
	Due        int           `json:"due_monitors"`
	Results    []CheckResult `json:"results,omitempty"`
	Message    string        `json:"message,omitempty"`
}

func Execute(ctx context.Context, o Options) (Output, error) {
	if o.DataDir == "" {
		o.DataDir = "data"
	}
	c, err := LoadConfig(o.ConfigPath)
	if err != nil {
		return Output{}, err
	}
	out := Output{Mode: "validate", ConfigPath: o.ConfigPath}
	if !o.Run {
		out.Message = "configuration is valid; pass -run to execute checks"
		return out, nil
	}
	if o.Now.IsZero() {
		o.Now = time.Now().UTC()
	}
	o.Now = o.Now.UTC()
	states := map[string]MonitorState{}
	statePath := filepath.Join(o.DataDir, "state", "monitor_states.json")
	if b, e := os.ReadFile(statePath); e == nil {
		if e = json.Unmarshal(b, &states); e != nil {
			return Output{}, fmt.Errorf("decode monitor states: %w", e)
		}
	} else if !os.IsNotExist(e) {
		return Output{}, e
	}
	threshold := c.Settings.Tolerance
	tolerance := 0 * time.Second
	if threshold != "" {
		tolerance, _ = time.ParseDuration(threshold)
	}
	failLimit := c.Settings.FailureThreshold
	if failLimit == 0 {
		failLimit = 3
	}
	recoverLimit := c.Settings.RecoveryThreshold
	if recoverLimit == 0 {
		recoverLimit = 2
	}
	incidents, err := loadIncidents(o.DataDir)
	if err != nil {
		return Output{}, err
	}
	schedules, err := loadSchedules(o.DataDir)
	if err != nil {
		return Output{}, err
	}
	monitorIDs := map[string]bool{}
	for _, m := range c.Monitors {
		monitorIDs[m.ID] = true
	}
	for _, w := range schedules {
		if w.Scope == "monitor" && !monitorIDs[w.MonitorID] {
			return Output{}, fmt.Errorf("maintenance %s references unknown monitor %q", w.ID, w.MonitorID)
		}
	}
	outbox, err := loadOutbox(o.DataDir)
	if err != nil {
		return Output{}, err
	}
	// Flush deferred maintenance effects before new check events so issue creation
	// precedes updates or resolution from this run.
	for i := range incidents {
		inc := &incidents[i]
		if !inc.NeedsIssue || maintenanceFor(schedules, inc.MonitorID, o.Now) != nil {
			continue
		}
		if inc.State == "open" {
			outbox = appendEvent(outbox, incidentEvent("incident_created", *inc, inc.LastError, o.Now))
		} else {
			outbox = appendEvent(outbox, incidentEvent("incident_resolved", *inc, "", o.Now))
		}
		inc.NeedsIssue = false
	}
	out.Mode = "run"
	for _, m := range c.Monitors {
		if !m.Enabled {
			continue
		}
		state := states[m.ID]
		activeMaintenance := maintenanceFor(schedules, m.ID, o.Now)
		if activeMaintenance != nil && activeMaintenance.Mode == "skip" {
			continue
		}
		interval, _ := time.ParseDuration(m.Interval)
		if state.LastRun != nil && !o.Force && o.Now.Sub(*state.LastRun) < interval-tolerance {
			continue
		}
		out.Due++
		timeout := 10 * time.Second
		if m.Timeout != "" {
			timeout, _ = time.ParseDuration(m.Timeout)
		}
		checker := o.Check
		if checker == nil {
			checker = check
		}
		r := checker(ctx, m, timeout)
		out.Results = append(out.Results, r)
		state.LastRun = &r.FinishedAt
		state = advanceState(state, r.Status, failLimit, recoverLimit)
		previousState := states[m.ID].State
		if previousState != "outage" && state.State == "outage" {
			incident := newIncident(m, o.Now)
			incident.LastError = r.Error
			if activeMaintenance != nil {
				incident.NeedsIssue = true
			}
			incidents = append(incidents, incident)
			if activeMaintenance == nil {
				outbox = appendEvent(outbox, incidentEvent("incident_created", incident, r.Error, o.Now))
			}
		}
		if previousState == "outage" && state.State == "outage" && activeMaintenance == nil {
			if i := findOpenIncident(incidents, m.ID); i >= 0 {
				outbox = appendEvent(outbox, incidentEvent("incident_updated", incidents[i], r.Error, o.Now))
			}
		}
		if previousState == "outage" && state.State == "outage" && activeMaintenance != nil {
			if i := findOpenIncident(incidents, m.ID); i >= 0 {
				incidents[i].LastError = r.Error
			}
		}
		if previousState == "outage" && state.State == "operational" {
			if i := findOpenIncident(incidents, m.ID); i >= 0 {
				incidents[i].State = "resolved"
				incidents[i].ResolvedAt = &r.FinishedAt
				if activeMaintenance != nil {
					incidents[i].NeedsIssue = true
				} else {
					outbox = appendEvent(outbox, incidentEvent("incident_resolved", incidents[i], "", r.FinishedAt))
				}
			}
		}
		states[m.ID] = state
	}
	for i := range schedules {
		w := &schedules[i]
		if !w.StartNotified && !o.Now.Before(w.StartsAt) {
			w.StartNotified = true
			outbox = appendEvent(outbox, maintenanceEvent("maintenance_started", *w, o.Now))
		}
		if w.StartNotified && !w.EndNotified && !o.Now.Before(w.EndsAt) {
			w.EndNotified = true
			outbox = appendEvent(outbox, maintenanceEvent("maintenance_ended", *w, o.Now))
		}
	}
	if out.Due == 0 {
		out.Message = "no enabled monitors are due"
	}
	if err := persist(out, states, o.Now, o.DataDir); err != nil {
		return Output{}, err
	}
	if err := writeJSON(filepath.Join(o.DataDir, "incidents", "active.json"), incidents); err != nil {
		return Output{}, fmt.Errorf("save incidents: %w", err)
	}
	if err := writeJSON(filepath.Join(o.DataDir, "outbox", "pending.json"), outbox); err != nil {
		return Output{}, fmt.Errorf("save outbox: %w", err)
	}
	if len(schedules) > 0 {
		if err := writeJSON(filepath.Join(o.DataDir, "maintenance", "schedules.json"), MaintenanceFile{SchemaVersion: 1, Schedules: schedules}); err != nil {
			return Output{}, fmt.Errorf("save maintenance schedules: %w", err)
		}
	}
	return out, nil
}

func persist(out Output, states map[string]MonitorState, now time.Time, dataDir string) error {
	if len(out.Results) == 0 {
		return nil
	}
	statePath := filepath.Join(dataDir, "state", "monitor_states.json")
	report := Report{SchemaVersion: 1, GeneratedAt: now, Checks: out.Results}
	day := now.Format("2006/01/02")
	paths := map[string]any{statePath: states, filepath.Join(dataDir, "state", "current.json"): out.Results}
	// Append a day's checks instead of replacing earlier workflow runs.
	historyPath := filepath.Join(dataDir, "history", day+".json")
	var existing Report
	if b, err := os.ReadFile(historyPath); err == nil {
		if err = json.Unmarshal(b, &existing); err != nil {
			return fmt.Errorf("decode history: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	existing.SchemaVersion = 1
	existing.GeneratedAt = now
	existing.Checks = append(existing.Checks, report.Checks...)
	paths[historyPath] = existing
	return atomicWriteJSON(paths)
}

func advanceState(s MonitorState, status string, failLimit, recoverLimit int) MonitorState {
	switch status {
	case "failure", "timeout":
		s.FailureCount++
		s.RecoveryCount = 0
		if s.FailureCount >= failLimit {
			s.State = "outage"
		} else if s.State == "" {
			s.State = "unknown"
		}
	case "success":
		s.FailureCount = 0
		if s.State == "outage" {
			s.RecoveryCount++
			if s.RecoveryCount >= recoverLimit {
				s.State = "operational"
				s.RecoveryCount = 0
			}
		} else {
			s.State = "operational"
		}
	}
	return s
}

func atomicWriteJSON(values map[string]any) error {
	temps := map[string]string{}
	for path, value := range values {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		f, err := os.CreateTemp(filepath.Dir(path), ".inaly-pulse-*.tmp")
		if err != nil {
			return err
		}
		if _, err = f.Write(b); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return err
		}
		if err = f.Sync(); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return err
		}
		if err = f.Close(); err != nil {
			_ = os.Remove(f.Name())
			return err
		}
		temps[path] = f.Name()
	}
	for path, tmp := range temps {
		if err := os.Rename(tmp, path); err != nil {
			for _, left := range temps {
				_ = os.Remove(left)
			}
			return err
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
