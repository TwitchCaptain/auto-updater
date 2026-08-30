//go:build windows

package host

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
)

func stop(path string) error {
	want, err := filepath.Abs(path)
	if err != nil {
		want = path
	}

	procs, err := process.Processes()
	if err != nil {
		return err
	}

	self := os.Getpid()
	base := strings.ToLower(filepath.Base(path))

	for _, p := range procs {
		if int(p.Pid) == self {
			continue
		}

		exe, err := p.Exe()
		if err != nil {
			name, nerr := p.Name()
			if nerr != nil || !strings.EqualFold(name, base) {
				continue
			}
		} else if !sameExe(exe, want, base) {
			continue
		}

		_ = p.Kill()
	}

	_ = stopService(want)

	return nil
}

func findService(exe string) string {
	want, err := filepath.Abs(exe)
	if err != nil {
		want = exe
	}

	ps := fmt.Sprintf(`Get-CimInstance Win32_Service | Where-Object { $_.PathName -and $_.PathName.ToLower().Contains('%s') } | Select-Object -ExpandProperty Name`,
		escapePS(strings.ToLower(want)))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	name := strings.TrimSpace(strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")[0])

	return name
}

func stopServiceName(name string) error {
	if name == "" {
		return nil
	}

	cmd := exec.Command("sc.exe", "stop", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()

	return nil
}

func startServiceName(name string) error {
	if name == "" {
		return nil
	}

	cmd := exec.Command("sc.exe", "start", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	return cmd.Run()
}

func scheduleReplace(src, dest string) error {
	script := fmt.Sprintf(`ping -n 3 127.0.0.1 >nul & move /Y "%s" "%s" & start "" "%s"`, src, dest, dest)
	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}

	return cmd.Start()
}

func sameExe(got, wantAbs, base string) bool {
	g, err := filepath.Abs(got)
	if err != nil {
		g = got
	}

	return strings.EqualFold(g, wantAbs) || strings.EqualFold(filepath.Base(g), base)
}

func countMatching(path string) (int, error) {
	want, _ := filepath.Abs(path)
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

		exe, err := p.Exe()
		if err != nil {
			continue
		}

		if sameExe(exe, want, base) {
			n++
		}
	}

	return n, nil
}

func startWindows(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "cmd.exe", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	return cmd.Start()
}

func startPath(ctx context.Context, path string) error {
	return startWindows(ctx, path)
}

func ParseShortcut(path string) (Shortcut, error) {
	ps := `$s = (New-Object -ComObject WScript.Shell).CreateShortcut('` + escapePS(path) + `'); Write-Output $s.TargetPath; Write-Output $s.Arguments; Write-Output $s.WorkingDirectory; Write-Output $s.IconLocation; Write-Output $s.Description`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.Output()
	if err != nil {
		return Shortcut{}, err
	}

	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	for len(lines) < 5 {
		lines = append(lines, "")
	}

	return Shortcut{
		Target:      strings.TrimSpace(lines[0]),
		Arguments:   strings.TrimSpace(lines[1]),
		WorkingDir:  strings.TrimSpace(lines[2]),
		Icon:        strings.TrimSpace(lines[3]),
		Description: strings.TrimSpace(lines[4]),
	}, nil
}

func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func elevateCopy(ctx context.Context, src, dest string) error {
	script := fmt.Sprintf(`Copy-Item -LiteralPath '%s' -Destination '%s' -Force`, escapePS(src), escapePS(dest))
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command",
		"Start-Process powershell.exe -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command',"+
			"'"+escapePS(script)+"'")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var buf bytes.Buffer
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("elevated copy: %w: %s", err, buf.String())
	}

	return nil
}

func stopService(exe string) error {
	return stopServiceName(findService(exe))
}

func SetStartup(enable bool, exe string) error {
	startup, err := windows.KnownFolderPath(windows.FOLDERID_Startup, 0)
	if err != nil {
		return err
	}

	lnk := filepath.Join(startup, "Captain Updater.lnk")
	if !enable {
		_ = os.Remove(lnk)

		return nil
	}

	ps := fmt.Sprintf(`$s = (New-Object -ComObject WScript.Shell).CreateShortcut('%s'); $s.TargetPath = '%s'; $s.WorkingDirectory = '%s'; $s.Save()`,
		escapePS(lnk), escapePS(exe), escapePS(filepath.Dir(exe)))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	return cmd.Run()
}
