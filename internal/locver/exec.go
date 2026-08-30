package locver

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var versionRe = regexp.MustCompile(`(?i)v?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.]+)?)`)

// versionCmd runs exe with args and returns combined output. Tests replace this.
var versionCmd = runVersionCmd

func fromCLI(ctx context.Context, exe string) (string, error) {
	for _, args := range [][]string{{"-v"}, {"--version"}} {
		out, err := versionCmd(ctx, exe, args)
		if v := parseVersion(out); v != "" {
			return v, nil
		}

		_ = err
	}

	return "", errors.New("no version from cli")
}

func runVersionCmd(ctx context.Context, exe string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, args...)
	hideWindow(cmd)
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()

	return string(out), err
}

func parseVersion(out string) string {
	m := versionRe.FindStringSubmatch(out)
	if len(m) < 2 {
		return ""
	}

	return strings.TrimPrefix(m[1], "v")
}
