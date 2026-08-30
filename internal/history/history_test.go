package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendTailRotate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	log := New(filepath.Join(dir, "history.jsonl"))

	if err := log.Append(Event{AppID: "a", Action: ActionCheck, Result: "ok"}); err != nil {
		t.Fatal(err)
	}

	got, err := log.Tail(10)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].AppID != "a" {
		t.Fatalf("%+v", got)
	}

	orig := maxEvents
	maxEvents = 3
	t.Cleanup(func() { maxEvents = orig })

	for i := range 5 {
		if err := log.Append(Event{AppID: "b", Action: ActionUpgrade, To: "1.0.0", Time: time.Unix(int64(i), 0)}); err != nil {
			t.Fatal(err)
		}
	}

	got, err = log.Tail(50)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 3 {
		t.Fatalf("len %d after rotate", len(got))
	}

	if got[0].Time.Unix() != 2 {
		t.Fatalf("expected oldest kept unix=2, got %v", got[0].Time)
	}

	if _, err := os.Stat(log.path); err != nil {
		t.Fatal(err)
	}
}

func TestTailMissingFile(t *testing.T) {
	t.Parallel()

	log := New(filepath.Join(t.TempDir(), "missing.jsonl"))
	got, err := log.Tail(10)
	if err != nil {
		t.Fatal(err)
	}

	if got == nil {
		t.Fatal("expected empty slice, not nil")
	}

	if len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}
