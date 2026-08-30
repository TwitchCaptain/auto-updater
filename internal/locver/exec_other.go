//go:build !windows

package locver

import "os/exec"

func hideWindow(_ *exec.Cmd) {}
