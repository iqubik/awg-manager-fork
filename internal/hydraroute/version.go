package hydraroute

import (
	"context"
	"regexp"
	"strings"
	"time"

	sysexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
)

var (
	versionProbeTimeout = 2 * time.Second                                                        //nolint:gochecknoglobals
	versionPattern      = regexp.MustCompile(`(?i)\bv?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`) //nolint:gochecknoglobals
)

func detectVersion(ctx context.Context) string {
	path := activeBinaryPath()
	if path == "" {
		return ""
	}
	commands := [][]string{
		{path, "--version"},
		{path, "version"},
		{path, "-v"},
	}
	for _, c := range commands {
		result, err := sysexec.RunWithOptions(ctx, c[0], c[1:], sysexec.Options{Timeout: versionProbeTimeout})
		if err != nil {
			continue
		}
		out := strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
		if v := parseVersionOutput(out); v != "" {
			return v
		}
	}
	return ""
}

func parseVersionOutput(out string) string {
	m := versionPattern.FindStringSubmatch(strings.TrimSpace(out))
	if len(m) < 2 {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(m[1]), "v")
}
