// Package mode is wlbrowser's vim-style modal key dispatcher. It owns the
// current mode (Normal/Insert/Command), a count buffer for vim-style numeric
// prefixes (5j), and a Trie walker that holds in-progress key sequences. It
// has no GTK dependencies — main.go wires it to the EventControllerKey.
package mode

import (
	"strconv"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"

	"github.com/dnslee/wlbrowser/internal/config"
	"github.com/dnslee/wlbrowser/internal/keys"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
)

// Dispatcher is the side-effect interface invoked when a binding fires.
type Dispatcher interface {
	Run(action config.Action, count int)
}

// Cancellable represents a scheduled timeout that can be cancelled.
type Cancellable interface{ Cancel() }

// Scheduler runs f on the main thread after d has elapsed. Production wires
// this to time.AfterFunc + glib.IdleAdd; tests use a fake.
type Scheduler interface {
	AfterFunc(d time.Duration, f func()) Cancellable
}

// Machine is the dispatcher state held between keystrokes.
type Machine struct {
	mu         sync.Mutex
	mode       Mode
	trie       *keys.Trie[config.Action]
	walker     *keys.Walker[config.Action]
	count      string
	pending    *config.Action
	timer      Cancellable
	timeout    time.Duration
	scheduler  Scheduler
	dispatcher Dispatcher
}

// New builds a Machine from a normal-mode binding table.
func New(bindings config.BindingTable, timeout time.Duration, sched Scheduler, disp Dispatcher) (*Machine, error) {
	tr := keys.NewTrie[config.Action]()
	for raw, action := range bindings {
		seq, err := keys.Parse(raw)
		if err != nil {
			return nil, err
		}
		tr.Add(seq, action)
	}
	return &Machine{
		mode:       ModeNormal,
		trie:       tr,
		walker:     tr.Walk(),
		timeout:    timeout,
		scheduler:  sched,
		dispatcher: disp,
	}, nil
}

func (m *Machine) Mode() Mode {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode
}

// SetMode is called by external triggers (focus tracker for Insert, cmdbar
// open/close for Command).
func (m *Machine) SetMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mode != ModeNormal {
		m.cancelLocked()
		m.count = ""
		m.walker.Reset()
	}
	m.mode = mode
}

// Feed processes one key event. Returns true if the event was consumed and
// should not propagate to WebKit.
func (m *Machine) Feed(tok keys.Token) bool {
	var actionToRun *config.Action
	var countToRun int
	var consumed bool

	m.mu.Lock()

	switch m.mode {
	case ModeInsert:
		if tok.Mods == 0 && tok.Key == gdk.KEY_Escape {
			// Caller (main.go) blurs the active element and flips back to
			// Normal mode via SetMode after our return.
			actionToRun = &config.Action{Name: "leave-insert"}
			countToRun = 1
			consumed = true
		} else if tok.Mods&(gdk.ControlMask|gdk.AltMask|gdk.SuperMask) != 0 {
			// Allow single-token modifier bindings (Ctrl/Alt/Super) through the
			// trie so global shortcuts like <C-k> work while a form field has
			// focus. Plain keys still pass to the page so typing is undisturbed.
			w := m.trie.Walk()
			if status, val := w.Feed(tok); status == keys.StatusComplete {
				actionToRun = &val
				countToRun = 1
				consumed = true
			}
		}
	case ModeCommand:
		// cmdbar entry consumes its own keys; we should not see anything here.
	case ModeNormal:
		// Count accumulation: digits 1-9 always start a count; 0 only continues one.
		if m.walker.AtRoot() && tok.Mods == 0 && !consumed {
			if d := digitOf(tok.Key); d >= 0 {
				if d > 0 || m.count != "" {
					m.count += strconv.Itoa(d)
					consumed = true
				}
			}
		}

		if !consumed {
			// New key arrives — cancel any pending timeout from a previous ambiguous
			// match; if the next feed is StatusNoMatch we'll discard it.
			m.cancelLocked()

			status, val := m.walker.Feed(tok)
			switch status {
			case keys.StatusComplete:
				actionToRun, countToRun = m.prepareFireLocked(val)
				consumed = true
			case keys.StatusAmbiguous:
				v := val
				m.pending = &v
				m.timer = m.scheduler.AfterFunc(m.timeout, m.timeoutFire)
				consumed = true
			case keys.StatusPartial:
				consumed = true
			case keys.StatusNoMatch:
				m.count = ""
			}
		}
	}

	m.mu.Unlock()

	if actionToRun != nil {
		m.dispatcher.Run(*actionToRun, countToRun)
	}

	return consumed
}

func (m *Machine) timeoutFire() {
	var actionToRun *config.Action
	var countToRun int

	m.mu.Lock()
	if m.pending != nil {
		val := *m.pending
		actionToRun, countToRun = m.prepareFireLocked(val)
	}
	m.mu.Unlock()

	if actionToRun != nil {
		m.dispatcher.Run(*actionToRun, countToRun)
	}
}

func (m *Machine) prepareFireLocked(action config.Action) (*config.Action, int) {
	count := 1
	if m.count != "" {
		if n, err := strconv.Atoi(m.count); err == nil && n > 0 {
			count = n
		}
	}
	m.count = ""
	m.pending = nil
	m.walker.Reset()
	return &action, count
}

func (m *Machine) cancelLocked() {
	if m.timer != nil {
		m.timer.Cancel()
		m.timer = nil
	}
	m.pending = nil
}

func digitOf(key uint) int {
	if key >= '0' && key <= '9' {
		return int(key - '0')
	}
	return -1
}

// RealScheduler wraps time.AfterFunc with a `runOnMain` callback so the timer
// fire ends up on the GTK main thread.
type RealScheduler struct {
	RunOnMain func(func())
}

type realCancel struct{ t *time.Timer }

func (r *realCancel) Cancel() {
	if r.t != nil {
		r.t.Stop()
	}
}

func (r RealScheduler) AfterFunc(d time.Duration, f func()) Cancellable {
	t := time.AfterFunc(d, func() {
		if r.RunOnMain != nil {
			r.RunOnMain(f)
		} else {
			f()
		}
	})
	return &realCancel{t: t}
}
