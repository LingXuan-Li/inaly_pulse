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
}
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
	out.Mode = "run"
	for _, m := range c.Monitors {
		if !m.Enabled {
			continue
		}
		state := states[m.ID]
		interval, _ := time.ParseDuration(m.Interval)
		if state.LastRun != nil && !o.Force && o.Now.Sub(*state.LastRun) < interval-tolerance {
			continue
		}
		out.Due++
		timeout := 10 * time.Second
		if m.Timeout != "" {
			timeout, _ = time.ParseDuration(m.Timeout)
		}
		r := check(ctx, m, timeout)
		out.Results = append(out.Results, r)
		state.LastRun = &r.FinishedAt
		state = advanceState(state, r.Status, failLimit, recoverLimit)
		states[m.ID] = state
	}
	if out.Due == 0 {
		out.Message = "no enabled monitors are due"
	}
	if err := persist(out, states, o.Now, o.DataDir); err != nil {
		return Output{}, err
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
