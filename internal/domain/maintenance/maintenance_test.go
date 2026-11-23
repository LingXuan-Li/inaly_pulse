package maintenance

import (
	"testing"
	"time"
)

func TestWindowStateAt(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := Window{StartsAt: start, EndsAt: start.Add(time.Hour)}

	cases := []struct {
		name string
		at   time.Time
		want State
	}{
		{"before start", start.Add(-time.Second), StateInactive},
		{"at start (inclusive)", start, StateActive},
		{"inside", start.Add(30 * time.Minute), StateActive},
		{"at end (exclusive)", start.Add(time.Hour), StateInactive},
		{"after end", start.Add(2 * time.Hour), StateInactive},
	}
	for _, c := range cases {
		if got := w.StateAt(c.at); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestWindowAppliesTo(t *testing.T) {
	global := Window{Scope: ScopeGlobal}
	if !global.AppliesTo("a") || !global.AppliesTo("b") {
		t.Error("global window should apply to every monitor")
	}
	one := Window{Scope: ScopeMonitor, MonitorID: "a"}
	if !one.AppliesTo("a") {
		t.Error("monitor window should apply to its own monitor")
	}
	if one.AppliesTo("b") {
		t.Error("monitor window should not apply to other monitors")
	}
	if (Window{}).AppliesTo("a") {
		t.Error("window without scope should not apply")
	}
}

func TestDefaultModeIsSuppress(t *testing.T) {
	if DefaultMode != ModeSuppress {
		t.Errorf("DefaultMode = %q, want %q", DefaultMode, ModeSuppress)
	}
}
