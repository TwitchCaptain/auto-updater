package history

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"
)

var maxEvents = 5000

const (
	ActionCheck   = "check"
	ActionUpgrade = "upgrade"
	ActionNotify  = "notify"
	ActionError   = "error"
)

// Event is one persisted history line.
type Event struct {
	Time    time.Time `json:"time"`
	AppID   string    `json:"appId"`
	AppName string    `json:"appName"`
	Action  string    `json:"action"`
	From    string    `json:"from,omitempty"`
	To      string    `json:"to,omitempty"`
	Asset   string    `json:"asset,omitempty"`
	Result  string    `json:"result,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// Log appends JSONL events and rotates at maxEvents.
type Log struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Log {
	return &Log{path: path}
}

func (l *Log) Append(ev Event) error {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.appendLine(ev); err != nil {
		return err
	}

	events, err := l.readAll()
	if err != nil {
		return err
	}

	if len(events) <= maxEvents {
		return nil
	}

	return l.rewrite(events[len(events)-maxEvents:])
}

func (l *Log) Tail(n int) ([]Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	events, err := l.readAll()
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}

		return nil, err
	}

	if events == nil {
		events = []Event{}
	}

	if n <= 0 || n >= len(events) {
		return events, nil
	}

	return events[len(events)-n:], nil
}

func (l *Log) appendLine(ev Event) error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(ev)
}

func (l *Log) readAll() ([]Event, error) {
	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		events = append(events, ev)
	}

	return events, sc.Err()
}

func (l *Log) rewrite(events []Event) error {
	tmp := l.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)

			return err
		}
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)

		return err
	}

	return os.Rename(tmp, l.path)
}
