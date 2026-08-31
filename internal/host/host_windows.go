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
	"unsafe"

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

	for _, p := range procs {
		if int(p.Pid) == self {
			continue
		}

		exe, err := p.Exe()
		if err != nil {
			continue
		}

		if !samePath(exe, want) {
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
	// $PID is a read-only automatic variable in PowerShell; do not assign it.
	script := fmt.Sprintf(`
$src = '%s'
$dest = '%s'
$waitPid = %d
$deadline = (Get-Date).AddMinutes(2)
while (Get-Process -Id $waitPid -ErrorAction SilentlyContinue) {
  if ((Get-Date) -gt $deadline) { break }
  Start-Sleep -Milliseconds 250
}
$dir = Split-Path -Parent $dest
if ($dir) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
$copied = $false
for ($i = 0; $i -lt 40; $i++) {
  try {
    Copy-Item -LiteralPath $src -Destination $dest -Force -ErrorAction Stop
    $copied = $true
    break
  } catch {
    Start-Sleep -Milliseconds 250
  }
}
if (-not $copied) { exit 1 }
Remove-Item -LiteralPath $src -ErrorAction SilentlyContinue
Start-Process -FilePath $dest
Remove-Item -LiteralPath $PSCommandPath -ErrorAction SilentlyContinue
`, escapePS(src), escapePS(dest), os.Getpid())

	ps1, err := writeTempPS1(script)
	if err != nil {
		return err
	}

	args := []string{"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-File", ps1}
	if !canWrite(filepath.Dir(dest)) {
		cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
			fmt.Sprintf(`Start-Process -FilePath powershell.exe -Verb RunAs -WindowStyle Hidden -ArgumentList @('-NoProfile','-NonInteractive','-File','%s')`, escapePS(ps1)))
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		}

		return cmd.Start()
	}

	cmd := exec.Command("powershell.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}

	return cmd.Start()
}

func writeTempPS1(script string) (string, error) {
	f, err := os.CreateTemp("", "cu-host-*.ps1")
	if err != nil {
		return "", err
	}

	name := f.Name()
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		_ = os.Remove(name)

		return "", err
	}

	return name, f.Close()
}

func countMatching(path string) (int, error) {
	want, _ := filepath.Abs(path)
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

		if samePath(exe, want) {
			n++
		}
	}

	return n, nil
}

func startPath(_ context.Context, path string) error {
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}

	var dir *uint16
	if !strings.EqualFold(filepath.Ext(path), ".lnk") {
		dir, _ = windows.UTF16PtrFromString(filepath.Dir(path))
	}

	return windows.ShellExecute(0, verb, file, nil, dir, windows.SW_SHOWNORMAL)
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
		Target:      strings.TrimSpace(strings.Trim(lines[0], `"'`)),
		Arguments:   strings.TrimSpace(lines[1]),
		WorkingDir:  strings.TrimSpace(strings.Trim(lines[2], `"'`)),
		Icon:        strings.TrimSpace(lines[3]),
		Description: strings.TrimSpace(lines[4]),
	}, nil
}

func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func elevateCopyMany(ctx context.Context, files []FileCopy) error {
	if len(files) == 0 {
		return nil
	}

	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "New-Item -ItemType Directory -Force -Path '%s' | Out-Null\n", escapePS(filepath.Dir(f.Dest)))
		fmt.Fprintf(&b, "Copy-Item -LiteralPath '%s' -Destination '%s' -Force\n", escapePS(f.Src), escapePS(f.Dest))
		fmt.Fprintf(&b, "if (-not $?) { exit 1 }\n")
	}

	ps1, err := writeTempPS1(b.String())
	if err != nil {
		return err
	}
	defer os.Remove(ps1)

	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		fmt.Sprintf(`$p = Start-Process -FilePath powershell.exe -Verb RunAs -Wait -PassThru -WindowStyle Hidden -ArgumentList @('-NoProfile','-NonInteractive','-File','%s'); if ($null -eq $p) { exit 1 }; exit $p.ExitCode`, escapePS(ps1)))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var buf bytes.Buffer
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("elevated copy: %w: %s", err, buf.String())
	}

	return nil
}

// SetAppUserModelID registers the process so Windows toasts can activate this app.
func SetAppUserModelID() error {
	p, err := windows.UTF16PtrFromString("TwitchCaptain.CaptainUpdater")
	if err != nil {
		return err
	}

	proc := windows.NewLazySystemDLL("shell32.dll").NewProc("SetCurrentProcessExplicitAppUserModelID")
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(p)))
	if r != 0 {
		return fmt.Errorf("SetCurrentProcessExplicitAppUserModelID: HRESULT 0x%x", r)
	}

	return nil
}

func stopService(exe string) error {
	return stopServiceName(findService(exe))
}

func hasStartup() bool {
	startup, err := windows.KnownFolderPath(windows.FOLDERID_Startup, 0)
	if err != nil {
		return false
	}

	_, err = os.Stat(filepath.Join(startup, "Captain Updater.lnk"))

	return err == nil
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
