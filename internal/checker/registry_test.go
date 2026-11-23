package checker

import (
	"context"
	"testing"

	"inaly_pulse/internal/domain/monitor"
)

type fakeChecker struct{ t monitor.Type }

func (f fakeChecker) Type() monitor.Type { return f.t }

func (f fakeChecker) Check(ctx context.Context, m monitor.Monitor) monitor.Result {
	return monitor.Result{MonitorID: m.ID, Type: f.t, Status: monitor.CheckSuccess}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeChecker{monitor.TypeHTTP}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	c, ok := r.Get(monitor.TypeHTTP)
	if !ok || c.Type() != monitor.TypeHTTP {
		t.Fatalf("Get(http) = %v, %v", c, ok)
	}
	if _, ok := r.Get(monitor.TypeTCP); ok {
		t.Error("Get(tcp) should not be found")
	}
}

func TestRegistryRejectsDuplicateNilAndEmpty(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeChecker{monitor.TypeDNS}); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := r.Register(fakeChecker{monitor.TypeDNS}); err == nil {
		t.Error("duplicate Register should fail")
	}
	if err := r.Register(nil); err == nil {
		t.Error("nil Register should fail")
	}
	if err := r.Register(fakeChecker{""}); err == nil {
		t.Error("empty type Register should fail")
	}
}

func TestRegistryTypesSorted(t *testing.T) {
	r := NewRegistry()
	for _, ty := range []monitor.Type{monitor.TypePing, monitor.TypeDNS, monitor.TypeHTTP, monitor.TypeTCP} {
		if err := r.Register(fakeChecker{ty}); err != nil {
			t.Fatalf("Register(%s): %v", ty, err)
		}
	}
	got := r.Types()
	want := []monitor.Type{monitor.TypeDNS, monitor.TypeHTTP, monitor.TypePing, monitor.TypeTCP}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
