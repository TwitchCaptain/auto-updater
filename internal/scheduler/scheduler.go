package scheduler

import (
	"sync"
	"time"

	"github.com/TwitchCaptain/auto-updater/internal/config"
	"github.com/TwitchCaptain/auto-updater/internal/schedule"
)

// Job is invoked when a slot is due.
type Job func(app config.App, slot schedule.Slot)

type Runner struct {
	store *config.Store
	job   Job
	mu    sync.Mutex
	fired map[string]string // key -> date-minute
	stop  chan struct{}
}

func New(store *config.Store, job Job) *Runner {
	return &Runner{store: store, job: job, fired: map[string]string{}, stop: make(chan struct{})}
}

func (r *Runner) Start() {
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

			key := app.ID + ":" + string(rune('0'+i))
			r.mu.Lock()
			if r.fired[key] == stamp {
				r.mu.Unlock()

				continue
			}

			r.fired[key] = stamp
			r.mu.Unlock()

			go r.job(app, slot)
		}
	}
}
