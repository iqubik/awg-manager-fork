package testing

import "time"

const (
	ConnectivityTimeout = 5 * time.Second
	IPCheckTimeout      = 15 * time.Second
)

// IPResult from test/ip endpoint
type IPResult struct {
	DirectIP    string     `json:"directIp"`
	VpnIP       string     `json:"vpnIp"`
	EndpointIP  string     `json:"endpointIp"`
	IPChanged   bool       `json:"ipChanged"`
	DirectGeo   *IPGeoInfo `json:"directGeo,omitempty"`
	VpnGeo      *IPGeoInfo `json:"vpnGeo,omitempty"`
	EndpointGeo *IPGeoInfo `json:"endpointGeo,omitempty"`
}

type IPGeoInfo struct {
	IP          string `json:"ip,omitempty"`
	Location    string `json:"location,omitempty"`
	City        string `json:"city,omitempty"`
	Region      string `json:"region,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	ISP         string `json:"isp,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	Source      string `json:"source,omitempty"`
}

// ConnectivityResult from test/connectivity endpoint
type ConnectivityResult struct {
	Connected bool   `json:"connected"`
	Latency   *int   `json:"latency,omitempty"`
	Reason    string `json:"reason,omitempty"`
	HTTPCode  *int   `json:"httpCode,omitempty"`
}

const (
	ReasonTunnelNotRunning   = "tunnel_not_running"
	ReasonConnectionFailed   = "connection_failed"
	ReasonUnexpectedResponse = "unexpected_response"
)

// IPCheckService is a public IP detection service.
type IPCheckService struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// SpeedTestResult from test/speed endpoint.
type SpeedTestResult struct {
	Server      string  `json:"server"`
	Direction   string  `json:"direction"` // "download" or "upload"
	Bandwidth   float64 `json:"bandwidth"` // Mbps
	Bytes       int64   `json:"bytes"`
	Duration    float64 `json:"duration"` // seconds
	Retransmits int     `json:"retransmits"`
}

// SpeedTestServer is a public iperf3 server.
type SpeedTestServer struct {
	Label string `json:"label"`
	Host  string `json:"host"`
	Port  int    `json:"port"`
}

// SpeedTestInfo returns availability and server list.
type SpeedTestInfo struct {
	Available bool              `json:"available"`
	Servers   []SpeedTestServer `json:"servers"`
}
