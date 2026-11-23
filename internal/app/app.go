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
	statePath := filepath.Join("data", "state", "monitor_states.json")
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
		switch r.Status {
		case "failure", "timeout":
			state.FailureCount++
			state.RecoveryCount = 0
			if state.FailureCount >= failLimit {
				state.State = "outage"
			} else if state.State == "" {
				state.State = "unknown"
			}
		case "success":
			state.FailureCount = 0
			if state.State == "outage" {
				state.RecoveryCount++
				if state.RecoveryCount >= recoverLimit {
					state.State = "operational"
					state.RecoveryCount = 0
				}
			} else {
				state.State = "operational"
			}
		}
		states[m.ID] = state
	}
	if out.Due == 0 {
		out.Message = "no enabled monitors are due"
	}
	if err := persist(out, states, o.Now); err != nil {
		return Output{}, err
	}
	return out, nil
}

func persist(out Output, states map[string]MonitorState, now time.Time) error {
	if len(out.Results) == 0 {
		return nil
	}
	statePath := filepath.Join("data", "state", "monitor_states.json")
	if err := writeJSON(statePath, states); err != nil {
		return fmt.Errorf("save states: %w", err)
	}
	if err := writeJSON(filepath.Join("data", "state", "current.json"), out.Results); err != nil {
		return fmt.Errorf("save current results: %w", err)
	}
	report := Report{SchemaVersion: 1, GeneratedAt: now, Checks: out.Results}
	day := now.Format("2006/01/02")
	if err := writeJSON(filepath.Join("data", "history", day+".json"), report); err != nil {
		return fmt.Errorf("save history: %w", err)
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
