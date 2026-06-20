package installer

// InstallState classifies the HydraRoute package install state for the
// settings UI.
type InstallState string

const (
	InstallStateInstalled       InstallState = "installed"
	InstallStateMissing         InstallState = "missing"
	InstallStateMissingNoSpace  InstallState = "missing_no_space"
	InstallStateOutdatedNoSpace InstallState = "outdated_no_space"
	InstallStateInstalling      InstallState = "installing"
	InstallStateError           InstallState = "error"
)
