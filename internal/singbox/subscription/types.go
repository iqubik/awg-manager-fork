package subscription

import "time"

// MemberInfo is parsed metadata for one selector member, persisted alongside
// the subscription so the UI can display useful info per card without
// re-parsing the raw outbound JSON each time.
type MemberInfo struct {
	Tag       string `json:"tag"`
	Protocol  string `json:"protocol"`            // "vless" | "trojan" | "shadowsocks" | "hysteria2" | "naive"
	Server    string `json:"server"`              // host
	Port      uint16 `json:"port"`
	Transport string `json:"transport,omitempty"` // "tcp" | "ws" | "grpc" | "http" — empty if N/A
	Security  string `json:"security,omitempty"`  // "tls" | "reality" | "" (none/Hy2 always tls)
}

// Subscription is the persisted shape of a VPN subscription.
type Subscription struct {
	ID             string       `json:"id"`                      // uuid
	Label          string       `json:"label"`                   // user-facing
	URL            string       `json:"url"`                     // subscription URL
	Headers        []Header     `json:"headers"`                 // custom HTTP headers for fetch
	RefreshHours   int          `json:"refreshHours"`            // 0 = manual only
	LastFetched    time.Time    `json:"lastFetched"`
	LastError      string       `json:"lastError,omitempty"`
	SelectorTag    string       `json:"selectorTag"`             // "sub-<id-short>"
	InboundTag     string       `json:"inboundTag"`              // "sub-<id-short>-in"
	ListenPort     uint16       `json:"listenPort"`              // localhost port for the mixed inbound
	ProxyIndex     int          `json:"proxyIndex"`              // NDMS ProxyN index, -1 if not yet allocated
	MemberTags     []string     `json:"memberTags"`              // every member outbound tag (kept for back-compat)
	Members        []MemberInfo `json:"members,omitempty"`       // per-member parsed metadata
	OrphanTags     []string     `json:"orphanTags"`              // tags missing on last refresh
	ActiveMember   string       `json:"activeMember,omitempty"` // currently-active selector member tag
	Enabled        bool         `json:"enabled"`
	IsDefaultRoute bool         `json:"isDefaultRoute"`
}

// Header is a single name:value pair sent on the fetch request.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CreateInput is the input to Service.Create.
type CreateInput struct {
	Label        string
	URL          string
	Headers      []Header
	RefreshHours int
	Enabled      bool
}

// UpdatePatch is partial update; nil pointers mean "leave as-is".
type UpdatePatch struct {
	Label        *string
	URL          *string
	Headers      *[]Header
	RefreshHours *int
	Enabled      *bool
}

// RefreshResult is the outcome of a single refresh cycle.
type RefreshResult struct {
	When         time.Time
	Err          error
	Added        int
	Updated      int
	Orphaned     int
	SkippedVmess int
	SkippedOther int
	ParseErrors  []string
}
