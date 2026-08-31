package scheduler

import (
	"strconv"
	"sync"
	"time"

	"github.com/TwitchCaptain/auto-updater/internal/config"
	"github.com/TwitchCaptain/auto-updater/internal/schedule"
)

// Job is invoked when a slot is due.
type Job func(app config.App, slot schedule.Slot)

// Handled reports whether this occurrence at dueAt already ran (history),
// so a start-up catch-up should skip it.
type Handled func(app config.App, slot schedule.Slot, dueAt time.Time) bool

type Runner struct {
	store   *config.Store
	job     Job
	Handled Handled
	mu      sync.Mutex
	fired   map[string]string // key -> date-minute
	stop    chan struct{}
}

func New(store *config.Store, job Job) *Runner {
	return &Runner{store: store, job: job, fired: map[string]string{}, stop: make(chan struct{})}
}

func (r *Runner) Start() {
	r.catchUp(time.Now())
	go r.loop()
}

func (r *Runner) Stop() {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
}

func (r *Runner) loop() {
	t := time.NewTicker(time.Second * 20)
	defer t.Stop()

	r.tick()

	for {
		select {
		case <-r.stop:
			return
		case <-t.C:
			r.tick()
		}
	}
}

func (r *Runner) tick() {
	now := time.Now()
	stamp := now.Format("2006-01-02 15:04")
	set := r.store.Settings()

	for _, app := range set.Apps {
		if !app.Enabled {
			continue
		}

		for i, slot := range app.Schedules {
			if !schedule.Due(slot, now) {
				continue
			}

			if !r.mark(slotKey(app.ID, i), stamp) {
				continue
			}

			go r.job(app, slot)
		}
	}
}

// catchUp runs the most recent missed occurrence of each slot when the tray
// was not running at that minute. The current minute is left to tick().
func (r *Runner) catchUp(now time.Time) {
	set := r.store.Settings()

	for _, app := range set.Apps {
		if !app.Enabled {
			continue
		}

		for i, slot := range app.Schedules {
			if schedule.Due(slot, now) {
				continue
			}

			prev, err := schedule.Previous(slot, now)
			if err != nil {
				continue
			}

			key := slotKey(app.ID, i)
			stamp := prev.Format("2006-01-02 15:04")
			if r.Handled != nil && r.Handled(app, slot, prev) {
				r.mark(key, stamp)

				continue
			}

			if !r.mark(key, stamp) {
				continue
			}

			go r.job(app, slot)
		}
	}
}

func (r *Runner) mark(key, stamp string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.fired[key] == stamp {
		return false
	}

	r.fired[key] = stamp

	return true
}

func slotKey(appID string, i int) string {
	return appID + ":" + strconv.Itoa(i)
}
