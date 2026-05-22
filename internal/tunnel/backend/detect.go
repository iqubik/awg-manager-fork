package backend

import (
	"os"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/sys/kmod"
)

// IsKernelAvailable checks if the AmneziaWG kernel module is loaded.
func IsKernelAvailable() bool {
	_, err := os.Stat(kmod.SysfsPath)
	return err == nil
}

// waitForKernel polls for the kernel module sysfs entry with a timeout.
func waitForKernel(timeout time.Duration) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if IsKernelAvailable() {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-ticker.C:
		}
	}
}

// New creates a kernel backend. appLog is optional (nil is safe).
func New(appLog *logging.ScopedLogger) Backend {
	if IsKernelAvailable() {
		appLog.Info("backend-detect", "", "using kernel backend")
		return NewKernel()
	}

	// Module may still be registering after insmod — wait with retry
	appLog.Info("backend-detect", "", "waiting for kernel module")
	if waitForKernel(5 * time.Second) {
		appLog.Info("backend-detect", "", "using kernel backend (after wait)")
		return NewKernel()
	}

	appLog.Warn("backend-detect", "", "kernel module not available — tunnel operations may fail")
	return NewKernel()
}
