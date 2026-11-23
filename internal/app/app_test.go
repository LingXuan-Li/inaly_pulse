package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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
