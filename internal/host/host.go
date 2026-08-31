package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Shortcut is a parsed .lnk (best-effort).
type Shortcut struct {
	Target      string `json:"target"`
	Arguments   string `json:"arguments"`
	WorkingDir  string `json:"workingDir"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

func Stop(path string) error {
	return stop(path)
}

func Start(ctx context.Context, exe, shortcut string) error {
	if shortcut != "" {
		return startPath(ctx, shortcut)
	}

	if exe == "" {
		return errors.New("nothing to start")
	}

	return startPath(ctx, exe)
}

func IsSelf(exePath string) bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}

	return samePath(exePath, self)
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil && strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB)) {
		return true
	}

	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	if err1 == nil && err2 == nil {
		return os.SameFile(fa, fb)
	}

	return false
}

// RunningCount is how many other processes are this exe (not this PID).
func RunningCount(path string) (int, error) {
	return countMatching(path)
}

func WaitStopped(path string, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		n, err := countMatching(path)
		if err != nil {
			return err
		}

		if n == 0 {
			return nil
		}

		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("process still running: %s", filepath.Base(path))
}

// FileCopy is one src → dest pair for PlaceMany.
type FileCopy struct {
	Src  string
	Dest string
}

// Place copies src onto dest, elevating with UAC when the dest is not writable.
func Place(ctx context.Context, src, dest string) error {
	return PlaceMany(ctx, []FileCopy{{Src: src, Dest: dest}})
}

// PlaceMany copies files, prompting UAC at most once for the whole batch.
func PlaceMany(ctx context.Context, files []FileCopy) error {
	var elevate []FileCopy

	for _, f := range files {
		if f.Src == "" || f.Dest == "" {
			continue
		}

		if !CanWrite(filepath.Dir(f.Dest)) {
			elevate = append(elevate, f)

			continue
		}

		if err := copyOverwrite(f.Src, f.Dest); err != nil {
			if isAccess(err) {
				elevate = append(elevate, f)

				continue
			}

			return err
		}
	}

	if len(elevate) == 0 {
		return nil
	}

	return ElevateCopyMany(ctx, elevate)
}

func ElevateCopy(ctx context.Context, src, dest string) error {
	return ElevateCopyMany(ctx, []FileCopy{{Src: src, Dest: dest}})
}

func ElevateCopyMany(ctx context.Context, files []FileCopy) error {
	return elevateCopyMany(ctx, files)
}

func copyOverwrite(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	tmp := dest + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}

	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)

		return err
	}

	return nil
}

func isAccess(err error) bool {
	if err == nil {
		return false
	}

	s := strings.ToLower(err.Error())

	return strings.Contains(s, "access is denied") || strings.Contains(s, "permission denied")
}

func canWrite(dir string) bool {
	if dir == "" {
		return false
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}

	f, err := os.CreateTemp(dir, ".cu-write-*")
	if err != nil {
		return false
	}

	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)

	return true
}

// CanWrite reports whether destDir (or its parent) is writable by this process.
func CanWrite(destDir string) bool {
	return canWrite(destDir)
}

// FindService returns a Windows service name whose image path is exe, if any.
func FindService(exe string) string {
	return findService(exe)
}

func StopService(name string) error {
	return stopServiceName(name)
}

func StartService(name string) error {
	return startServiceName(name)
}

// ScheduleReplace replaces dest with src after this process exits (self-update).
func ScheduleReplace(src, dest string) error {
	return scheduleReplace(src, dest)
}

// HasStartup reports whether the current-user Startup folder has our shortcut.
func HasStartup() bool {
	return hasStartup()
}
