package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigRejectsUnknownFieldsAndPrivateTargets(t *testing.T) {
	for _, target := range []string{"http://127.0.0.1/", "http://localhost/", "http://10.0.0.1/"} {
		c := Config{SchemaVersion: 1, Monitors: []MonitorConfig{{ID: "x", Name: "x", Type: "http", Enabled: true, Target: target, Interval: "10m"}}}
		if c.Validate() == nil {
			t.Errorf("expected unsafe target rejection: %s", target)
		}
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"unknown":true,"monitors":[{"id":"x","name":"x","type":"http","enabled":true,"target":"https://example.com","interval":"10m"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestHTTPCheckerAuthentication(t *testing.T) {
	t.Setenv("INALY_MONITOR_SECRETS", `{"CREDS":{"type":"basic","username":"alice","password":"secret"}}`)
	request, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	m := MonitorConfig{ID: "x", Type: "http", Authentication: &AuthenticationConfig{SecretRef: "CREDS"}}
	if err := applyAuthentication(request, m); err != nil {
		t.Fatal(err)
	}
	user, password, ok := request.BasicAuth()
	if !ok || user != "alice" || password != "secret" {
		t.Fatal("configured basic authentication was not applied")
	}
}

func TestHTTPCheckerRejectsPrivateTargets(t *testing.T) {
	for _, target := range []string{"http://127.0.0.1/", "http://localhost/", "http://192.168.1.1/"} {
		m := MonitorConfig{ID: "x", Type: "http", Target: target}
		if err := validateMonitorURL(target); err == nil {
			t.Errorf("validateMonitorURL(%s) accepted", target)
		}
		_ = m
	}
}

func TestAdvanceStateHysteresis(t *testing.T) {
	s := MonitorState{State: "operational"}
	for i := 0; i < 2; i++ {
		s = advanceState(s, "failure", 3, 2)
		if s.State != "operational" {
			t.Fatalf("failure %d changed state: %#v", i+1, s)
		}
	}
	s = advanceState(s, "failure", 3, 2)
	if s.State != "outage" {
		t.Fatalf("third failure: %#v", s)
	}
	s = advanceState(s, "success", 3, 2)
	if s.State != "outage" || s.RecoveryCount != 1 {
		t.Fatalf("first recovery: %#v", s)
	}
	s = advanceState(s, "success", 3, 2)
	if s.State != "operational" {
		t.Fatalf("second recovery: %#v", s)
	}
}

func TestMaintenanceScopeAndMode(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ws := []MaintenanceWindow{{ID: "global", Scope: "global", Mode: "suppress", StartsAt: start, EndsAt: start.Add(time.Hour)}, {ID: "one", Scope: "monitor", MonitorID: "a", Mode: "skip", StartsAt: start, EndsAt: start.Add(time.Hour)}}
	if w := maintenanceFor(ws, "a", start); w == nil || w.ID != "one" {
		t.Fatalf("monitor skip should take precedence: %#v", w)
	}
	if w := maintenanceFor(ws, "b", start); w == nil || w.ID != "global" {
		t.Fatalf("global maintenance not applied: %#v", w)
	}
	if w := maintenanceFor(ws, "a", start.Add(time.Hour)); w != nil {
		t.Fatal("maintenance end should be exclusive")
	}
}

func TestLoadSchedulesValidatesAndDefaultsMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "maintenance", "schedules.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data := `{"schema_version":1,"schedules":[{"id":"m1","scope":"global","origin":"scheduled","starts_at":"2026-01-01T00:00:00Z","ends_at":"2026-01-01T01:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	ws, err := loadSchedules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 1 {
		t.Fatalf("got %d schedules", len(ws))
	}
	if ws[0].Mode != "suppress" {
		t.Fatalf("default mode=%q", ws[0].Mode)
	}
}

func TestExecuteCreatesOutageAndDefersMaintenanceEffects(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "monitors.json")
	config := `{"schema_version":1,"monitors":[{"id":"site","name":"Site","type":"http","enabled":true,"target":"https://example.com","interval":"10m"}],"settings":{"failure_threshold":1,"recovery_threshold":2}}`
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	now := time.Date(2026, 1, 1, 0, 30, 0, 0, time.UTC)
	sched := MaintenanceFile{SchemaVersion: 1, Schedules: []MaintenanceWindow{{ID: "window", Scope: "global", Origin: "scheduled", Mode: "suppress", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)}}}
	if err := writeJSON(filepath.Join(dataDir, "maintenance", "schedules.json"), sched); err != nil {
		t.Fatal(err)
	}
	checkFn := func(_ context.Context, m MonitorConfig, _ time.Duration) CheckResult {
		return CheckResult{MonitorID: m.ID, Type: m.Type, Status: "failure", StartedAt: now, FinishedAt: now.Add(time.Second), Error: "unavailable"}
	}
	if _, err := Execute(context.Background(), Options{ConfigPath: configPath, DataDir: dataDir, Run: true, Force: true, Now: now, Check: checkFn}); err != nil {
		t.Fatal(err)
	}
	incidents, err := loadIncidents(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 || incidents[0].State != "open" || !incidents[0].NeedsIssue {
		t.Fatalf("maintenance incident=%#v", incidents)
	}
	events, err := loadOutbox(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if strings.HasPrefix(e.Type, "incident_") {
			t.Fatalf("incident event must be suppressed during maintenance: %#v", e)
		}
	}
	ended := now.Add(2 * time.Hour)
	if _, err := Execute(context.Background(), Options{ConfigPath: configPath, DataDir: dataDir, Run: true, Force: true, Now: ended, Check: checkFn}); err != nil {
		t.Fatal(err)
	}
	events, err = loadOutbox(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	createdIndex, updatedIndex := -1, -1
	for i, e := range events {
		if e.Type == "incident_created" && createdIndex < 0 {
			createdIndex = i
		}
		if e.Type == "incident_updated" && updatedIndex < 0 {
			updatedIndex = i
		}
	}
	if createdIndex < 0 || updatedIndex < createdIndex {
		t.Fatalf("deferred issue creation should precede outage update: %#v", events)
	}
}

func TestPersistenceAppendsHistoryAtomically(t *testing.T) {
	temp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(temp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	now := time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC)
	result := CheckResult{MonitorID: "x", Type: "http", Status: "success", StartedAt: now, FinishedAt: now.Add(time.Second)}
	for i := 0; i < 2; i++ {
		out := Output{Results: []CheckResult{result}}
		if err := persist(out, map[string]MonitorState{"x": {State: "operational"}}, now, "data"); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join("data", "history", "2026", "11", "23.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Report
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Checks) != 2 {
		t.Fatalf("history has %d checks", len(got.Checks))
	}
}
