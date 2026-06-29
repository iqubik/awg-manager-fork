package singboxwatchdog

import "time"

type TargetKind string

const (
	TargetTunnel       TargetKind = "tunnel"
	TargetSubscription TargetKind = "subscription"
)

type RecoveryMode string

const (
	RecoveryOff            RecoveryMode = "off"
	RecoveryRestartSingbox RecoveryMode = "restart-singbox"
	RecoverySwitchMember   RecoveryMode = "switch-member"
)

type StatusKind string

const (
	StatusAlive      StatusKind = "alive"
	StatusWarming    StatusKind = "warming"
	StatusRecovering StatusKind = "recovering"
	StatusDead       StatusKind = "dead"
	StatusDisabled   StatusKind = "disabled"
	StatusStopped    StatusKind = "stopped"
)

type TargetConfig struct {
	ID            string       `json:"id"`
	Kind          TargetKind   `json:"kind"`
	Ref           string       `json:"ref"`
	Enabled       bool         `json:"enabled"`
	Interval      int          `json:"interval"`
	FailThreshold int          `json:"failThreshold"`
	Timeout       int          `json:"timeout"`
	RecoveryMode  RecoveryMode `json:"recoveryMode"`
	PersistSwitch bool         `json:"persistSwitch,omitempty"`
}

type RuntimeTarget struct {
	ID              string
	Kind            TargetKind
	Ref             string
	Name            string
	CheckTag        string
	TrafficTag      string
	SelectorTag     string
	ActiveMemberTag string
	MemberTags      []string
	ListenPort      int
	Protocol        string
	Security        string
	Transport       string
	ProxyInterface  string
	KernelInterface string
	Running         bool
	Config          *TargetConfig
}

type TargetStatus struct {
	ID              string       `json:"id"`
	Kind            TargetKind   `json:"kind"`
	Ref             string       `json:"ref"`
	Name            string       `json:"name"`
	CheckTag        string       `json:"checkTag"`
	TrafficTag      string       `json:"trafficTag"`
	SelectorTag     string       `json:"selectorTag,omitempty"`
	ActiveMemberTag string       `json:"activeMemberTag,omitempty"`
	Protocol        string       `json:"protocol,omitempty"`
	Security        string       `json:"security,omitempty"`
	Transport       string       `json:"transport,omitempty"`
	ProxyInterface  string       `json:"proxyInterface,omitempty"`
	KernelInterface string       `json:"kernelInterface,omitempty"`
	Running         bool         `json:"running"`
	Configured      bool         `json:"configured"`
	Enabled         bool         `json:"enabled"`
	Status          StatusKind   `json:"status"`
	Interval        int          `json:"interval"`
	Timeout         int          `json:"timeout"`
	LastCheck       *time.Time   `json:"lastCheck,omitempty"`
	LastLatency     int          `json:"lastLatency"`
	FailCount       int          `json:"failCount"`
	FailThreshold   int          `json:"failThreshold"`
	RestartCount    int          `json:"restartCount"`
	SwitchCount     int          `json:"switchCount"`
	LastError       string       `json:"lastError,omitempty"`
	LastRecovery    string       `json:"lastRecovery,omitempty"`
	RecoveryMode    RecoveryMode `json:"recoveryMode"`
	PersistSwitch   bool         `json:"persistSwitch,omitempty"`
}

type LogEntry struct {
	Timestamp   time.Time  `json:"timestamp"`
	TargetID    string     `json:"targetId"`
	TargetName  string     `json:"targetName"`
	Kind        TargetKind `json:"kind"`
	CheckTag    string     `json:"checkTag"`
	Success     bool       `json:"success"`
	Latency     int        `json:"latency"`
	Error       string     `json:"error"`
	FailCount   int        `json:"failCount"`
	Threshold   int        `json:"threshold"`
	StateChange string     `json:"stateChange"`
	MemberFrom  string     `json:"memberFrom,omitempty"`
	MemberTo    string     `json:"memberTo,omitempty"`
}

type CheckResult struct {
	Success bool
	Latency int
	Error   string
}
