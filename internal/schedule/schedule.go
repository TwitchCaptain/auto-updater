package schedule

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const MaxSlots = 5

const (
	ActionUpgrade = "upgrade"
	ActionNotify  = "notify"
)

// Slot is one weekly fire time. Days uses time.Weekday (Sunday=0).
type Slot struct {
	Days   []time.Weekday `json:"days"`
	Time   string         `json:"time"` // HH:MM 24h local
	Action string         `json:"action"`
}

func (s Slot) hourMinute() (int, int, error) {
	return parseHM(s.Time)
}

func parseHM(raw string) (int, int, error) {
	raw = strings.TrimSpace(raw)
	var h, m, sec int
	n, err := fmt.Sscanf(raw, "%d:%d:%d", &h, &m, &sec)
	if n < 2 {
		return 0, 0, fmt.Errorf("time %q: %w", raw, err)
	}

	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("time %q out of range", raw)
	}

	return h, m, nil
}

// CanonicalTime normalizes a UI time value to HH:MM (WebView2 may send HH:MM:SS).
func CanonicalTime(raw string) string {
	h, m, err := parseHM(raw)
	if err != nil {
		return raw
	}

	return fmt.Sprintf("%02d:%02d", h, m)
}

func (s Slot) hasDay(d time.Weekday) bool {
	return slices.Contains(s.Days, d)
}

func (s Slot) validAction() bool {
	return s.Action == ActionUpgrade || s.Action == ActionNotify
}

// Validate checks a list of slots for an app.
func Validate(slots []Slot) error {
	if len(slots) > MaxSlots {
		return fmt.Errorf("at most %d schedules", MaxSlots)
	}

	for i, s := range slots {
		if len(s.Days) == 0 {
			return fmt.Errorf("schedule %d: pick at least one day", i+1)
		}

		if _, _, err := s.hourMinute(); err != nil {
			return fmt.Errorf("schedule %d: %w", i+1, err)
		}

		if !s.validAction() {
			return fmt.Errorf("schedule %d: action must be %s or %s", i+1, ActionUpgrade, ActionNotify)
		}
	}

	return nil
}

// Due reports whether slot should fire at now (minute precision).
func Due(slot Slot, now time.Time) bool {
	if !slot.hasDay(now.Weekday()) {
		return false
	}

	h, m, err := slot.hourMinute()
	if err != nil {
		return false
	}

	return now.Hour() == h && now.Minute() == m
}

// Previous is the most recent fire at or before `from` (inclusive if currently due).
func Previous(slot Slot, from time.Time) (time.Time, error) {
	h, m, err := slot.hourMinute()
	if err != nil {
		return time.Time{}, err
	}

	if len(slot.Days) == 0 {
		return time.Time{}, errors.New("no days")
	}

	t := from.Truncate(time.Minute)

	for range 8 * 24 * 60 {
		if Due(slot, t) && t.Hour() == h && t.Minute() == m {
			return t, nil
		}

		t = t.Add(-time.Minute)
	}

	return time.Time{}, errors.New("no previous run")
}

// Next after `from` (exclusive of the current minute if already due).
func Next(slot Slot, from time.Time) (time.Time, error) {
	h, m, err := slot.hourMinute()
	if err != nil {
		return time.Time{}, err
	}

	if len(slot.Days) == 0 {
		return time.Time{}, errors.New("no days")
	}

	t := from.Truncate(time.Minute).Add(time.Minute)

	for range 8 * 24 * 60 {
		if Due(slot, t) && t.Hour() == h && t.Minute() == m {
			return t, nil
		}

		t = t.Add(time.Minute)
	}

	return time.Time{}, errors.New("no next run")
}

// Planned is the next fire of one slot.
type Planned struct {
	Time   time.Time
	Action string
}

// NextEach returns the next fire after from for every valid slot, soonest first.
func NextEach(slots []Slot, from time.Time) []Planned {
	out := make([]Planned, 0, len(slots))
	for _, slot := range slots {
		t, err := Next(slot, from)
		if err != nil {
			continue
		}

		out = append(out, Planned{Time: t, Action: slot.Action})
	}

	slices.SortFunc(out, func(a, b Planned) int {
		return a.Time.Compare(b.Time)
	})

	return out
}
