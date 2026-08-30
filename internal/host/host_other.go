//go:build !windows

package host

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func stop(path string) error {
	base := strings.ToLower(filepath.Base(path))
	procs, err := process.Processes()
	if err != nil {
		return err
	}

	self := os.Getpid()
	for _, p := range procs {
		if int(p.Pid) == self {
			continue
		}

		name, err := p.Name()
		if err != nil {
			continue
		}

		if strings.EqualFold(name, base) {
			_ = p.Kill()
		}
	}

	return nil
}

func countMatching(path string) (int, error) {
	base := strings.ToLower(filepath.Base(path))
	procs, err := process.Processes()
	if err != nil {
		return 0, err
	}

	n := 0
	self := os.Getpid()

	for _, p := range procs {
		if int(p.Pid) == self {
			continue
		}

		name, err := p.Name()
		if err != nil {
			continue
		}

		if strings.EqualFold(name, base) {
			n++
		}
	}

	return n, nil
}

func ParseShortcut(_ string) (Shortcut, error) {
	return Shortcut{}, errors.New("shortcuts are Windows-only")
}

func startPath(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, path)
	cmd.Dir = filepath.Dir(path)
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Start()
}

func elevateCopy(_ context.Context, _, _ string) error {
	return errors.New("UAC copy is Windows-only")
}

func SetStartup(_ bool, _ string) error {
	return nil
}

func findService(_ string) string { return "" }

func stopServiceName(_ string) error { return nil }

func startServiceName(_ string) error { return nil }

func scheduleReplace(src, dest string) error {
	_ = os.Remove(dest)

	return os.Rename(src, dest)
}
