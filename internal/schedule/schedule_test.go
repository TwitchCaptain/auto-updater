package schedule

import (
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	t.Parallel()

	slot := Slot{
		Days:   []time.Weekday{time.Monday, time.Wednesday},
		Time:   "03:15",
		Action: ActionUpgrade,
	}

	mon := time.Date(2026, 8, 31, 3, 15, 30, 0, time.UTC) // Monday
	if mon.Weekday() != time.Monday {
		t.Fatalf("fixture weekday %s", mon.Weekday())
	}

	if !Due(slot, mon) {
		t.Fatal("expected due")
	}

	tue := time.Date(2026, 9, 1, 3, 15, 0, 0, time.UTC)
	if Due(slot, tue) {
		t.Fatal("tuesday should not fire")
	}

	wrongTime := time.Date(2026, 8, 31, 3, 16, 0, 0, time.UTC)
	if Due(slot, wrongTime) {
		t.Fatal("wrong minute")
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	if err := Validate(nil); err != nil {
		t.Fatal(err)
	}

	tooMany := make([]Slot, MaxSlots+1)
	if err := Validate(tooMany); err == nil {
		t.Fatal("expected too many")
	}

	bad := []Slot{{Days: []time.Weekday{time.Friday}, Time: "25:00", Action: ActionNotify}}
	if err := Validate(bad); err == nil {
		t.Fatal("expected bad time")
	}
}

func TestNext(t *testing.T) {
	t.Parallel()

	slot := Slot{
		Days:   []time.Weekday{time.Saturday},
		Time:   "10:00",
		Action: ActionNotify,
	}

	from := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC) // Saturday before 10:00
	next, err := Next(slot, from)
	if err != nil {
		t.Fatal(err)
	}

	if next.Weekday() != time.Saturday || next.Hour() != 10 || next.Minute() != 0 {
		t.Fatalf("got %s", next)
	}

	after := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	next, err = Next(slot, after)
	if err != nil {
		t.Fatal(err)
	}

	if next.Weekday() != time.Saturday {
		t.Fatalf("got %s", next)
	}

	if next.Sub(after) < 6*24*time.Hour {
		t.Fatalf("expected next week, got %s", next)
	}
}

func TestCanonicalTime(t *testing.T) {
	t.Parallel()

	if CanonicalTime("4:00") != "04:00" {
		t.Fatal(CanonicalTime("4:00"))
	}

	if CanonicalTime("04:00:00") != "04:00" {
		t.Fatal(CanonicalTime("04:00:00"))
	}

	slot := Slot{Days: []time.Weekday{time.Monday}, Time: "03:15:00", Action: ActionUpgrade}
	if err := Validate([]Slot{slot}); err != nil {
		t.Fatal(err)
	}

	mon := time.Date(2026, 8, 31, 3, 15, 0, 0, time.UTC)
	if !Due(slot, mon) {
		t.Fatal("seconds from <input type=time> must still match the minute")
	}
}

func TestNextEach(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC) // Saturday
	slots := []Slot{
		{Days: []time.Weekday{time.Saturday}, Time: "18:00", Action: ActionNotify},
		{Days: []time.Weekday{time.Saturday}, Time: "10:00", Action: ActionUpgrade},
		{Days: []time.Weekday{}, Time: "11:00", Action: ActionNotify},
	}

	got := NextEach(slots, from)
	if len(got) != 2 {
		t.Fatalf("len %d: %+v", len(got), got)
	}

	if got[0].Action != ActionUpgrade || got[0].Time.Hour() != 10 {
		t.Fatalf("want 10:00 upgrade first, got %+v", got[0])
	}

	if got[1].Action != ActionNotify || got[1].Time.Hour() != 18 {
		t.Fatalf("want 18:00 notify second, got %+v", got[1])
	}
}
