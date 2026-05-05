package mode

import (
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"

	"github.com/dnslee/wlbrowser/internal/config"
	"github.com/dnslee/wlbrowser/internal/keys"
)

type recDispatch struct {
	calls []call
}
type call struct {
	name  string
	count int
}

func (r *recDispatch) Run(a config.Action, c int) {
	r.calls = append(r.calls, call{name: a.Name, count: c})
}

type fakeSched struct {
	pending []func()
}

func (f *fakeSched) AfterFunc(d time.Duration, fn func()) Cancellable {
	idx := len(f.pending)
	f.pending = append(f.pending, fn)
	return &fakeCancel{f: f, idx: idx}
}
func (f *fakeSched) flush() {
	for i, fn := range f.pending {
		if fn != nil {
			f.pending[i] = nil
			fn()
		}
	}
}

type fakeCancel struct {
	f   *fakeSched
	idx int
}

func (c *fakeCancel) Cancel() { c.f.pending[c.idx] = nil }

func newMachine(t *testing.T, bindings map[string]string) (*Machine, *recDispatch, *fakeSched) {
	t.Helper()
	d := &recDispatch{}
	s := &fakeSched{}
	bt := config.BindingTable{}
	for k, v := range bindings {
		bt[k] = config.Action{Name: v}
	}
	m, err := New(bt, 600*time.Millisecond, s, d)
	if err != nil {
		t.Fatal(err)
	}
	return m, d, s
}

func tk(mods gdk.ModifierType, key uint) keys.Token { return keys.Normalize(mods, key) }

func TestSimpleBinding(t *testing.T) {
	m, d, _ := newMachine(t, map[string]string{"j": "scroll-down"})
	if !m.Feed(tk(0, 'j')) {
		t.Fatal("expected j to be consumed")
	}
	if len(d.calls) != 1 || d.calls[0].name != "scroll-down" || d.calls[0].count != 1 {
		t.Errorf("got %+v", d.calls)
	}
}

func TestSequenceBinding(t *testing.T) {
	m, d, _ := newMachine(t, map[string]string{"gg": "scroll-top"})
	m.Feed(tk(0, 'g')) // partial
	if len(d.calls) != 0 {
		t.Errorf("dispatch on partial: %+v", d.calls)
	}
	m.Feed(tk(0, 'g')) // complete
	if len(d.calls) != 1 || d.calls[0].name != "scroll-top" {
		t.Errorf("got %+v", d.calls)
	}
}

func TestAmbiguousResolvesToLonger(t *testing.T) {
	m, d, _ := newMachine(t, map[string]string{"g": "g-alone", "gg": "scroll-top"})
	m.Feed(tk(0, 'g')) // ambiguous, schedules timeout
	m.Feed(tk(0, 'g')) // resolves to gg
	if len(d.calls) != 1 || d.calls[0].name != "scroll-top" {
		t.Errorf("got %+v", d.calls)
	}
}

func TestAmbiguousTimeoutFiresShorter(t *testing.T) {
	m, d, s := newMachine(t, map[string]string{"g": "g-alone", "gg": "scroll-top"})
	m.Feed(tk(0, 'g'))
	if len(d.calls) != 0 {
		t.Errorf("premature dispatch: %+v", d.calls)
	}
	s.flush() // simulate timeout
	if len(d.calls) != 1 || d.calls[0].name != "g-alone" {
		t.Errorf("got %+v", d.calls)
	}
}

func TestCount(t *testing.T) {
	m, d, _ := newMachine(t, map[string]string{"j": "scroll-down"})
	m.Feed(tk(0, '5'))
	m.Feed(tk(0, 'j'))
	if len(d.calls) != 1 || d.calls[0].count != 5 {
		t.Errorf("got %+v", d.calls)
	}
	// Multi-digit.
	m.Feed(tk(0, '1'))
	m.Feed(tk(0, '2'))
	m.Feed(tk(0, 'j'))
	if d.calls[1].count != 12 {
		t.Errorf("got count %d, want 12", d.calls[1].count)
	}
}

func TestZeroIsBindable(t *testing.T) {
	// 0 alone (no count active) should fire its binding, not start a count.
	m, d, _ := newMachine(t, map[string]string{"0": "zoom-reset"})
	m.Feed(tk(0, '0'))
	if len(d.calls) != 1 || d.calls[0].name != "zoom-reset" {
		t.Errorf("got %+v", d.calls)
	}
}

func TestZeroExtendsCount(t *testing.T) {
	m, d, _ := newMachine(t, map[string]string{"j": "scroll-down"})
	m.Feed(tk(0, '1'))
	m.Feed(tk(0, '0'))
	m.Feed(tk(0, 'j'))
	if len(d.calls) != 1 || d.calls[0].count != 10 {
		t.Errorf("got %+v", d.calls)
	}
}

func TestUnboundPassesThrough(t *testing.T) {
	m, _, _ := newMachine(t, map[string]string{"j": "scroll-down"})
	if m.Feed(tk(0, 'x')) {
		t.Error("unbound key should not be consumed")
	}
}

func TestInsertModePassesThrough(t *testing.T) {
	m, d, _ := newMachine(t, map[string]string{"j": "scroll-down"})
	m.SetMode(ModeInsert)
	if m.Feed(tk(0, 'j')) {
		t.Error("Insert mode must not consume j")
	}
	if len(d.calls) != 0 {
		t.Errorf("Insert mode dispatched: %+v", d.calls)
	}
	// Esc fires leave-insert.
	if !m.Feed(tk(0, gdk.KEY_Escape)) {
		t.Error("Esc must be consumed in Insert mode")
	}
	if len(d.calls) != 1 || d.calls[0].name != "leave-insert" {
		t.Errorf("got %+v", d.calls)
	}
}
