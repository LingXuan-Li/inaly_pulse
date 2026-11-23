package incident

import "testing"

func TestStateActive(t *testing.T) {
	cases := map[State]bool{
		StateOpen:          true,
		StateInvestigating: true,
		StateResolved:      false,
		State("unknown"):   false,
	}
	for s, want := range cases {
		if got := s.Active(); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}

func TestStateValid(t *testing.T) {
	if !StateOpen.Valid() || !StateInvestigating.Valid() || !StateResolved.Valid() {
		t.Error("defined states should be valid")
	}
	if State("closed").Valid() {
		t.Error("undefined state should be invalid")
	}
}
