package scheduler

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/TwitchCaptain/auto-updater/internal/config"
	"github.com/TwitchCaptain/auto-updater/internal/schedule"
)

func testStore(t *testing.T, slot schedule.Slot) *config.Store {
	t.Helper()

	store := config.New(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Detect(); err != nil {
		t.Fatal(err)
	}

	err := store.Replace(config.Settings{
		Apps: []config.App{{
			ID:        "app-1",
			Name:      "unpackerr",
			Enabled:   true,
			OwnerRepo: "Unpackerr/unpackerr",
			ExePath:   `C:\apps\unpackerr.exe`,
			Schedules: []schedule.Slot{slot},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	return store
}

func TestCatchUpMissedSlot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.Local)
	slot := schedule.Slot{
		Days:   []time.Weekday{now.Weekday()},
		Time:   "03:15",
		Action: schedule.ActionUpgrade,
	}
	store := testStore(t, slot)

	var mu sync.Mutex
	var n int
	r := New(store, func(_ config.App, s schedule.Slot) {
		mu.Lock()
		n++
		mu.Unlock()

		if s.Action != schedule.ActionUpgrade {
			t.Errorf("action %s", s.Action)
		}
	})

	r.catchUp(now)
	if !waitCount(t, &mu, &n, 1) {
		return
	}

	r.catchUp(now)
	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	got := n
	mu.Unlock()
	if got != 1 {
		t.Fatalf("second catch-up should be a no-op, got %d", got)
	}
}

func TestCatchUpSkipsWhenHandled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.Local)
	slot := schedule.Slot{
		Days:   []time.Weekday{now.Weekday()},
		Time:   "03:15",
		Action: schedule.ActionNotify,
	}
	store := testStore(t, slot)

	var n int
	r := New(store, func(config.App, schedule.Slot) { n++ })
	r.Handled = func(config.App, schedule.Slot, time.Time) bool { return true }
	r.catchUp(now)
	time.Sleep(30 * time.Millisecond)
	if n != 0 {
		t.Fatalf("handled occurrence should not run, got %d", n)
	}
}

func TestCatchUpLeavesCurrentMinuteToTick(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 30, 3, 15, 20, 0, time.Local)
	slot := schedule.Slot{
		Days:   []time.Weekday{now.Weekday()},
		Time:   "03:15",
		Action: schedule.ActionUpgrade,
	}
	store := testStore(t, slot)

	var n int
	r := New(store, func(config.App, schedule.Slot) { n++ })
	r.catchUp(now)
	time.Sleep(30 * time.Millisecond)
	if n != 0 {
		t.Fatalf("current minute is tick's job, got %d", n)
	}
}

func waitCount(t *testing.T, mu *sync.Mutex, n *int, want int) bool {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := *n
		mu.Unlock()
		if got >= want {
			return true
		}

		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	got := *n
	mu.Unlock()
	t.Fatalf("jobs %d want %d", got, want)

	return false
}
