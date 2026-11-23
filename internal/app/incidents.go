package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func loadIncidents(dataDir string) ([]Incident, error) {
	var values []Incident
	err := readJSON(filepath.Join(dataDir, "incidents", "active.json"), &values)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load incidents: %w", err)
	}
	return values, nil
}
func loadOutbox(dataDir string) ([]OutboxEvent, error) {
	var values []OutboxEvent
	err := readJSON(filepath.Join(dataDir, "outbox", "pending.json"), &values)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load notification outbox: %w", err)
	}
	return values, nil
}
func newIncident(m MonitorConfig, at time.Time) Incident {
	id := newID()
	return Incident{ID: id, MonitorID: m.ID, MonitorName: m.Name, State: "open", OpenedAt: at}
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("incident-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:])
}
func findOpenIncident(xs []Incident, monitorID string) int {
	for i := len(xs) - 1; i >= 0; i-- {
		if xs[i].MonitorID == monitorID && xs[i].State == "open" {
			return i
		}
	}
	return -1
}
func appendEvent(xs []OutboxEvent, e OutboxEvent) []OutboxEvent {
	for _, old := range xs {
		if old.ID == e.ID {
			return xs
		}
	}
	return append(xs, e)
}
func incidentEvent(kind string, inc Incident, summary string, at time.Time) OutboxEvent {
	return OutboxEvent{ID: newID(), Type: kind, AggregateID: inc.ID, CreatedAt: at, Status: "pending", Payload: map[string]any{"monitor_id": inc.MonitorID, "monitor_name": inc.MonitorName, "incident_id": inc.ID, "summary": summary, "occurred_at": inc.OpenedAt, "resolved_at": inc.ResolvedAt, "issue_url": inc.IssueURL}}
}
func maintenanceEvent(kind string, w MaintenanceWindow, at time.Time) OutboxEvent {
	return OutboxEvent{ID: newID(), Type: kind, AggregateID: w.ID, CreatedAt: at, Status: "pending", Payload: map[string]any{"maintenance_id": w.ID, "scope": w.Scope, "monitor_id": w.MonitorID, "occurred_at": at}}
}
