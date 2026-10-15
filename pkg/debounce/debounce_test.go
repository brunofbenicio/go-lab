package debounce

import (
	"testing"
	"time"
)

type fakeTimer struct {
	due            time.Duration
	f              func()
	stopped, fired bool
}

func (t *fakeTimer) Stop() bool {
	active := !t.stopped && !t.fired
	t.stopped = true
	return active
}

// fakeTimers is a manual clock: timers only fire inside Advance.
type fakeTimers struct {
	now    time.Duration
	timers []*fakeTimer
}

func (ft *fakeTimers) AfterFunc(d time.Duration, f func()) Stopper {
	t := &fakeTimer{due: ft.now + d, f: f}
	ft.timers = append(ft.timers, t)
	return t
}

func (ft *fakeTimers) Advance(d time.Duration) {
	ft.now += d
	for _, t := range ft.timers {
		if !t.stopped && !t.fired && t.due <= ft.now {
			t.fired = true
			t.f()
		}
	}
}

func setup(wait time.Duration) (*Debouncer, *fakeTimers, *int) {
	ft, runs := &fakeTimers{}, new(int)
	return New(wait, func() { *runs++ }, WithAfterFunc(ft.AfterFunc)), ft, runs
}

func TestCoalescesCalls(t *testing.T) {
	const ms = time.Millisecond
	type step struct {
		call    bool
		advance time.Duration
		runs    int // total executions expected after the step
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"single call fires after wait", []step{{true, 0, 0}, {false, 99 * ms, 0}, {false, 1 * ms, 1}}},
		{"each call restarts the wait", []step{
			{true, 0, 0}, {true, 99 * ms, 0}, {true, 99 * ms, 0}, {false, 1 * ms, 1},
		}},
		{"fires once for a burst", []step{{true, 0, 0}, {true, 0, 0}, {true, 0, 0}, {false, 100 * ms, 1}, {false, time.Hour, 1}}},
		{"works again after firing", []step{{true, 0, 0}, {false, 100 * ms, 1}, {true, 0, 1}, {false, 100 * ms, 2}}},
		{"no call, no run", []step{{false, time.Hour, 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, ft, runs := setup(100 * ms)
			for i, s := range tt.steps {
				if s.call {
					d.Call()
				}
				ft.Advance(s.advance)
				if *runs != s.runs {
					t.Fatalf("step %d: runs = %d, want %d", i, *runs, s.runs)
				}
			}
		})
	}
}

func TestCancelAndFlush(t *testing.T) {
	d, ft, runs := setup(time.Second)

	if d.Cancel() || d.Flush() {
		t.Error("Cancel/Flush with nothing pending should report false")
	}
	d.Call()
	if !d.Cancel() {
		t.Error("Cancel should report true when a call was pending")
	}
	ft.Advance(time.Hour)
	if *runs != 0 {
		t.Errorf("runs = %d after Cancel, want 0", *runs)
	}

	d.Call()
	if !d.Flush() || *runs != 1 {
		t.Errorf("Flush = runs %d, want 1 immediately", *runs)
	}
	ft.Advance(time.Hour)
	if *runs != 1 {
		t.Errorf("runs = %d, the flushed call must not fire again", *runs)
	}
}

func TestStaleTimerCallbackIsIgnored(t *testing.T) {
	d, ft, runs := setup(time.Second)
	d.Call()
	stale := ft.timers[0].f // a timer that Stop could not prevent from running
	d.Call()
	stale()
	if *runs != 0 {
		t.Errorf("a superseded timer ran fn (%d runs)", *runs)
	}
	ft.Advance(time.Second)
	if *runs != 1 {
		t.Errorf("runs = %d, want 1", *runs)
	}
}
