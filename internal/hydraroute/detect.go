package hydraroute

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	defaultLegacyHrneoBinary = "/opt/bin/hrneo"
	defaultLegacyNeoCommand  = "/opt/bin/neo"
	defaultPIDFile           = "/var/run/hrneo.pid"
)

var (
	legacyHrneoBinary = defaultLegacyHrneoBinary //nolint:gochecknoglobals
	legacyNeoCommand  = defaultLegacyNeoCommand  //nolint:gochecknoglobals
	pidFile           = defaultPIDFile           //nolint:gochecknoglobals
)

type Paths struct {
	Binary  string
	Control string
}

func ResolvePaths() Paths {
	if isExecutableFile(legacyHrneoBinary) {
		return Paths{
			Binary:  legacyHrneoBinary,
			Control: legacyNeoCommand,
		}
	}
	return Paths{}
}

// Detect checks if HydraRoute Neo is installed and running.
func Detect() Status {
	s := Status{
		ProcessState: StateNotInstalled,
	}
	paths := ResolvePaths()
	s.Installed = paths.Binary != ""

	if !s.Installed {
		return s
	}
	s.ProcessState = StateStopped

	raw, err := os.ReadFile(pidFile)
	if err != nil {
		return s
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		s.ProcessState = StateDead
		return s
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		s.ProcessState = StateDead
		s.StalePID = pid
		return s
	}

	if err := proc.Signal(syscall.Signal(0)); err == nil {
		s.Running = true
		s.PID = pid
		s.ProcessState = StateRunning
		return s
	}
	s.ProcessState = StateDead
	s.StalePID = pid

	return s
}

func activeBinaryPath() string {
	return ResolvePaths().Binary
}

func activeControlPath() string {
	return ResolvePaths().Control
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func binaryFingerprint(path string) string {
	if path == "" {
		return ""
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return ""
	}
	return strings.Join([]string{
		filepath.Clean(path),
		st.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		st.Mode().String(),
		strconv.FormatInt(st.Size(), 10),
	}, "|")
}
