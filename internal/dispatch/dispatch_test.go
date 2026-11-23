package dispatch

import (
	"testing"
	"time"

	"inaly_pulse/internal/app"
)

func TestNotifyEnabledMaintenanceAlias(t *testing.T) {
	settings := map[string]bool{"maintenance": true}
	if !notifyEnabled(settings, "maintenance_started") || !notifyEnabled(settings, "maintenance_ended") {
		t.Fatal("maintenance alias must enable both maintenance notifications")
	}
	settings["maintenance_ended"] = false
	if notifyEnabled(settings, "maintenance_ended") {
		t.Fatal("event-specific setting should override alias")
	}
}
func TestNotifierEventUsesPayloadTimes(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	e := app.OutboxEvent{ID: "evt", Type: "incident_resolved", CreatedAt: at, Payload: map[string]any{"incident_id": "i-1", "monitor_name": "API", "occurred_at": at.Format(time.RFC3339Nano), "resolved_at": at.Add(time.Minute).Format(time.RFC3339Nano)}}
	got := toNotifierEvent(e)
	if got.IncidentID != "i-1" || got.MonitorName != "API" || got.ResolvedAt == nil || !got.ResolvedAt.Equal(at.Add(time.Minute)) {
		t.Fatalf("unexpected event: %#v", got)
	}
}
