package api

import (
	"testing"

	"github.com/hoaxisr/awg-manager/internal/hydraroute"
)

func TestHydraRouteStatusData_MapsAllFields(t *testing.T) {
	in := hydraroute.Status{
		Installed:    true,
		Running:      false,
		Version:      "2.4.1",
		PID:          1234,
		StalePID:     5678,
		ProcessState: hydraroute.StateDead,
		LastError:    "neo restart: exit status 1",
		Managed:      true,
		Legacy:       false,

		CurrentVersion:          "2.4.1",
		RequiredVersion:         "2.4.2",
		CurrentSHA256:           "aaa",
		RequiredSHA256:          "bbb",
		VersionMatchesRequired:  false,
		ChecksumMatchesRequired: false,
		CustomBuild:             false,
		UpdateAvailable:         true,
		InstallState:            "missing",
		RequiredBytes:           111,
		FreeBytes:               222,
		InstallSupported:        true,
	}

	got := hydraRouteStatusData(in)

	if got.Installed != in.Installed {
		t.Fatalf("Installed=%v want %v", got.Installed, in.Installed)
	}
	if got.Running != in.Running {
		t.Fatalf("Running=%v want %v", got.Running, in.Running)
	}
	if got.Version != in.Version {
		t.Fatalf("Version=%q want %q", got.Version, in.Version)
	}
	if got.PID != in.PID {
		t.Fatalf("PID=%d want %d", got.PID, in.PID)
	}
	if got.StalePID != in.StalePID {
		t.Fatalf("StalePID=%d want %d", got.StalePID, in.StalePID)
	}
	if got.ProcessState != string(in.ProcessState) {
		t.Fatalf("ProcessState=%q want %q", got.ProcessState, in.ProcessState)
	}
	if got.LastError != in.LastError {
		t.Fatalf("LastError=%q want %q", got.LastError, in.LastError)
	}
	if got.Managed != in.Managed || got.Legacy != in.Legacy {
		t.Fatalf("managed/legacy=%v/%v want %v/%v", got.Managed, got.Legacy, in.Managed, in.Legacy)
	}
	if got.CurrentVersion != in.CurrentVersion || got.RequiredVersion != in.RequiredVersion {
		t.Fatalf("version pair=%q/%q want %q/%q", got.CurrentVersion, got.RequiredVersion, in.CurrentVersion, in.RequiredVersion)
	}
	if got.CurrentSHA256 != in.CurrentSHA256 || got.RequiredSHA256 != in.RequiredSHA256 {
		t.Fatalf("sha pair=%q/%q want %q/%q", got.CurrentSHA256, got.RequiredSHA256, in.CurrentSHA256, in.RequiredSHA256)
	}
	if got.VersionMatchesRequired != in.VersionMatchesRequired || got.ChecksumMatchesRequired != in.ChecksumMatchesRequired {
		t.Fatalf("match flags=%v/%v want %v/%v", got.VersionMatchesRequired, got.ChecksumMatchesRequired, in.VersionMatchesRequired, in.ChecksumMatchesRequired)
	}
	if got.CustomBuild != in.CustomBuild || got.UpdateAvailable != in.UpdateAvailable {
		t.Fatalf("custom/update=%v/%v want %v/%v", got.CustomBuild, got.UpdateAvailable, in.CustomBuild, in.UpdateAvailable)
	}
	if got.InstallState != in.InstallState || got.RequiredBytes != in.RequiredBytes || got.FreeBytes != in.FreeBytes || got.InstallSupported != in.InstallSupported {
		t.Fatalf("install fields=%q/%d/%d/%v want %q/%d/%d/%v", got.InstallState, got.RequiredBytes, got.FreeBytes, got.InstallSupported, in.InstallState, in.RequiredBytes, in.FreeBytes, in.InstallSupported)
	}
}

func TestEvalNativewg(t *testing.T) {
	cases := []struct {
		name          string
		hasComponent  bool
		supportsASC   bool
		awgProxy      bool
		wantAvailable bool
		wantReason    string
	}{
		{"no wireguard component", false, false, false, false, nwgReasonNoComponent},
		{"no component even with proxy", false, false, true, false, nwgReasonNoComponent},
		{"component + ASC firmware", true, true, false, true, ""},
		{"component + awg_proxy obfuscation", true, false, true, true, ""},
		{"component but no obfuscation path", true, false, false, false, nwgReasonNoObfuscation},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			avail, reason := evalNativewg(c.hasComponent, c.supportsASC, c.awgProxy)
			if avail != c.wantAvailable {
				t.Fatalf("available=%v want %v", avail, c.wantAvailable)
			}
			if reason != c.wantReason {
				t.Fatalf("reason=%q want %q", reason, c.wantReason)
			}
		})
	}
}
