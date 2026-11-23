package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type MaintenanceWindow struct {
	ID            string    `json:"id"`
	Scope         string    `json:"scope"`
	MonitorID     string    `json:"monitor_id,omitempty"`
	Origin        string    `json:"origin"`
	Mode          string    `json:"mode,omitempty"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
	StartNotified bool      `json:"start_notified,omitempty"`
	EndNotified   bool      `json:"end_notified,omitempty"`
}
type MaintenanceFile struct {
	SchemaVersion int                 `json:"schema_version"`
	Schedules     []MaintenanceWindow `json:"schedules"`
}

func loadSchedules(dataDir string) ([]MaintenanceWindow, error) {
	var f MaintenanceFile
	path := filepath.Join(dataDir, "maintenance", "schedules.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load maintenance schedules: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("decode maintenance schedules: %w", err)
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("maintenance schedules must contain a single JSON value")
	}
	if f.SchemaVersion != 1 {
		return nil, fmt.Errorf("maintenance schedules schema_version must be 1")
	}
	seen := map[string]bool{}
	for i := range f.Schedules {
		w := &f.Schedules[i]
		if w.ID == "" || seen[w.ID] {
			return nil, fmt.Errorf("maintenance schedule IDs must be present and unique")
		}
		seen[w.ID] = true
		if w.Scope != "global" && w.Scope != "monitor" {
			return nil, fmt.Errorf("maintenance %s has invalid scope", w.ID)
		}
		if w.Scope == "monitor" && w.MonitorID == "" {
			return nil, fmt.Errorf("maintenance %s requires monitor_id", w.ID)
		}
		if w.Mode == "" {
			w.Mode = "suppress"
		}
		if w.Mode != "suppress" && w.Mode != "skip" {
			return nil, fmt.Errorf("maintenance %s has invalid mode", w.ID)
		}
		if !w.EndsAt.After(w.StartsAt) {
			return nil, fmt.Errorf("maintenance %s ends_at must be after starts_at", w.ID)
		}
		if w.Origin != "scheduled" && w.Origin != "manual" {
			return nil, fmt.Errorf("maintenance %s has invalid origin", w.ID)
		}
		_, offset := w.StartsAt.Zone()
		if offset != 0 {
			return nil, fmt.Errorf("maintenance %s starts_at must be UTC", w.ID)
		}
		_, offset = w.EndsAt.Zone()
		if offset != 0 {
			return nil, fmt.Errorf("maintenance %s ends_at must be UTC", w.ID)
		}
	}
	return f.Schedules, nil
}

func maintenanceFor(ws []MaintenanceWindow, monitorID string, at time.Time) *MaintenanceWindow {
	var suppress *MaintenanceWindow
	for i := range ws {
		w := &ws[i]
		if at.Before(w.StartsAt) || !at.Before(w.EndsAt) {
			continue
		}
		if w.Scope == "global" || w.Scope == "monitor" && w.MonitorID == monitorID {
			if w.Mode == "skip" {
				return w
			}
			if suppress == nil {
				suppress = w
			}
		}
	}
	return suppress
}

func readJSON(path string, target any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, target); err != nil {
		return err
	}
	return nil
}
