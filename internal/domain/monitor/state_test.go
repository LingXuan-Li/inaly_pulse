package monitor

import "testing"

func TestCheckStatusValid(t *testing.T) {
	for _, s := range []CheckStatus{CheckSuccess, CheckFailure, CheckTimeout, CheckUnsupported, CheckSkipped} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []CheckStatus{"", "ok", "SUCCESS"} {
		if s.Valid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestCheckStatusCountsAsFailure(t *testing.T) {
	cases := map[CheckStatus]bool{
		CheckSuccess:     false,
		CheckFailure:     true,
		CheckTimeout:     true,
		CheckUnsupported: false,
		CheckSkipped:     false,
	}
	for s, want := range cases {
		if got := s.CountsAsFailure(); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}

func TestStateValid(t *testing.T) {
	for _, s := range []State{StateUnknown, StateOperational, StateDegraded, StateOutage} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []State{"", "up", "down"} {
		if s.Valid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}
