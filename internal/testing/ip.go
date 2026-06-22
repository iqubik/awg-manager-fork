package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	neturl "net/url"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

var autoIPCheckService = IPCheckService{Label: "Auto (geo + fallback)", URL: ""}

var defaultGeoIPCheckService = IPCheckService{Label: "myip.wtf geo", URL: "http://myip.wtf/json"}

// defaultIPCheckServices is the built-in list of IP detection services.
var defaultIPCheckServices = []IPCheckService{
	{Label: "2ip", URL: "https://2ip.ru"},
	{Label: "wtfismyip", URL: "https://wtfismyip.com/text"},
	{Label: "ipinfo", URL: "https://ipinfo.io/ip"},
}

const (
	directIPTimeout   = 10 * time.Second
	vpnIPTimeout      = 20 * time.Second
	perServiceTimeout = 4 * time.Second
)

const endpointGeoLookupTemplate = "http://ip-api.com/json/%s?fields=status,message,query,country,countryCode,regionName,city,isp"

type myIPWTFResponse struct {
	IP          string `json:"YourFuckingIPAddress"`
	Location    string `json:"YourFuckingLocation"`
	Hostname    string `json:"YourFuckingHostname"`
	ISP         string `json:"YourFuckingISP"`
	City        string `json:"YourFuckingCity"`
	Country     string `json:"YourFuckingCountry"`
	CountryCode string `json:"YourFuckingCountryCode"`
}

type endpointGeoResponse struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Query       string `json:"query"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	RegionName  string `json:"regionName"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
}

// GetIPCheckServices returns the list of available IP check services.
func (s *Service) GetIPCheckServices() []IPCheckService {
	return append([]IPCheckService{autoIPCheckService, defaultGeoIPCheckService}, defaultIPCheckServices...)
}

// CheckIP tests if traffic goes through tunnel by comparing direct and VPN IPs.
// If serviceURL is non-empty, only that service is used (no fallback).
func (s *Service) CheckIP(ctx context.Context, tunnelID string, serviceURL string) (*IPResult, error) {
	if err := s.CheckTunnelRunning(tunnelID); err != nil {
		return nil, err
	}

	// Determine WAN interface for direct (non-VPN) check.
	var wanIface string
	if w := s.GetWANInterface(tunnelID); w != "" {
		wanIface = w
	}

	// Get direct IP (through WAN, bypassing tunnel default route).
	directCtx, directCancel := context.WithTimeout(ctx, directIPTimeout)
	defer directCancel()

	directProbe, err := s.fetchIPProbe(directCtx, serviceURL, wanIface)
	if err != nil {
		return nil, fmt.Errorf("failed to get WAN IP: %w", err)
	}

	// Get VPN IP (through tunnel).
	iface, err := s.GetInterfaceName(tunnelID)
	if err != nil {
		return nil, err
	}

	vpnCtx, vpnCancel := context.WithTimeout(ctx, vpnIPTimeout)
	defer vpnCancel()

	vpnProbe, err := s.fetchIPProbe(vpnCtx, serviceURL, iface)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP through tunnel: %w", err)
	}

	endpointIP := s.GetEndpointIP(tunnelID)
	endpointGeo := lookupEndpointGeo(ctx, endpointIP)

	return &IPResult{
		DirectIP:    directProbe.IP,
		VpnIP:       vpnProbe.IP,
		EndpointIP:  endpointIP,
		IPChanged:   directProbe.IP != vpnProbe.IP,
		DirectGeo:   geoInfoPtr(directProbe),
		VpnGeo:      geoInfoPtr(vpnProbe),
		EndpointGeo: endpointGeo,
	}, nil
}

// fetchIPProbe delegates to the standalone fetchIPProbe function.
func (s *Service) fetchIPProbe(ctx context.Context, serviceURL string, iface string) (IPGeoInfo, error) {
	return fetchIPProbe(ctx, serviceURL, iface)
}

// fetchIPProbeOne queries a single IP check service.
func fetchIPProbeOne(ctx context.Context, serviceURL string, iface string) (IPGeoInfo, error) {
	res, err := httpclient.DefaultClient.Do(ctx, httpclient.CallConfig{
		URL:       serviceURL,
		Interface: iface,
		MaxTime:   perServiceTimeout,
	})
	if err != nil {
		return IPGeoInfo{}, fmt.Errorf("%s: %w", serviceURL, err)
	}

	info, err := parseIPProbeBody(res.Body, serviceURL)
	if err != nil {
		return IPGeoInfo{}, fmt.Errorf("%s: %w", serviceURL, err)
	}
	return info, nil
}

// isValidIP checks if the string is a valid IPv4 or IPv6 address.
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}

func parseMyIPWTFJSON(body string) (IPGeoInfo, error) {
	var payload myIPWTFResponse
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return IPGeoInfo{}, err
	}
	info := IPGeoInfo{
		IP:          strings.TrimSpace(payload.IP),
		Location:    strings.TrimSpace(payload.Location),
		City:        strings.TrimSpace(payload.City),
		Country:     strings.TrimSpace(payload.Country),
		CountryCode: strings.TrimSpace(payload.CountryCode),
		ISP:         strings.TrimSpace(payload.ISP),
		Hostname:    strings.TrimSpace(payload.Hostname),
	}
	if !isValidIP(info.IP) {
		return IPGeoInfo{}, fmt.Errorf("invalid geo IP payload")
	}
	return info, nil
}

func parseGenericIPJSON(body string) (IPGeoInfo, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return IPGeoInfo{}, err
	}

	readString := func(keys ...string) string {
		for _, key := range keys {
			if v, ok := raw[key]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}

	info := IPGeoInfo{
		IP:          readString("ip", "query"),
		Location:    readString("location"),
		City:        readString("city"),
		Region:      readString("region", "regionName"),
		Country:     readString("country"),
		CountryCode: readString("countryCode", "country_code"),
		ISP:         readString("isp", "org"),
		Hostname:    readString("hostname"),
	}
	if !isValidIP(info.IP) {
		return IPGeoInfo{}, fmt.Errorf("invalid generic IP JSON payload")
	}
	return info, nil
}

func parseIPProbeBody(body string, source string) (IPGeoInfo, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return IPGeoInfo{}, fmt.Errorf("empty response")
	}

	if strings.HasPrefix(trimmed, "{") {
		if info, err := parseMyIPWTFJSON(trimmed); err == nil {
			info.Source = source
			return info, nil
		}
		if info, err := parseGenericIPJSON(trimmed); err == nil {
			info.Source = source
			return info, nil
		}
	}

	if isValidIP(trimmed) {
		return IPGeoInfo{IP: trimmed, Source: source}, nil
	}

	if strings.TrimSpace(source) != "" {
		return IPGeoInfo{}, fmt.Errorf("invalid IP check response from %s", source)
	}
	return IPGeoInfo{}, fmt.Errorf("invalid IP check response")
}

func composeLocation(info *IPGeoInfo) string {
	if info == nil {
		return ""
	}
	if v := strings.TrimSpace(info.Location); v != "" {
		return v
	}
	parts := make([]string, 0, 3)
	if v := strings.TrimSpace(info.City); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(info.Region); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(info.CountryCode); v != "" {
		parts = append(parts, v)
	} else if v := strings.TrimSpace(info.Country); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, ", ")
}

func FormatIPGeoSummary(info *IPGeoInfo) string {
	if info == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if loc := composeLocation(info); loc != "" {
		parts = append(parts, loc)
	}
	if isp := strings.TrimSpace(info.ISP); isp != "" {
		parts = append(parts, "ISP: "+isp)
	}
	return strings.Join(parts, ", ")
}

func geoInfoPtr(info IPGeoInfo) *IPGeoInfo {
	if strings.TrimSpace(info.IP) == "" &&
		strings.TrimSpace(info.Location) == "" &&
		strings.TrimSpace(info.City) == "" &&
		strings.TrimSpace(info.Region) == "" &&
		strings.TrimSpace(info.Country) == "" &&
		strings.TrimSpace(info.CountryCode) == "" &&
		strings.TrimSpace(info.ISP) == "" &&
		strings.TrimSpace(info.Hostname) == "" {
		return nil
	}
	copy := info
	return &copy
}

// CheckIPByInterface tests IP through a kernel interface directly.
// Used for system tunnels. Fetches both direct (WAN) and VPN IPs, compares them.
func CheckIPByInterface(ctx context.Context, ifaceName string, serviceURL string) (*IPResult, error) {
	// Get direct IP (without interface binding — through default route).
	directCtx, directCancel := context.WithTimeout(ctx, directIPTimeout)
	defer directCancel()

	directProbe, err := fetchIPProbe(directCtx, serviceURL, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get WAN IP: %w", err)
	}

	// Get VPN IP (through the specified interface).
	vpnCtx, vpnCancel := context.WithTimeout(ctx, vpnIPTimeout)
	defer vpnCancel()

	vpnProbe, err := fetchIPProbe(vpnCtx, serviceURL, ifaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP through interface: %w", err)
	}

	return &IPResult{
		DirectIP:  directProbe.IP,
		VpnIP:     vpnProbe.IP,
		IPChanged: directProbe.IP != vpnProbe.IP,
		DirectGeo: geoInfoPtr(directProbe),
		VpnGeo:    geoInfoPtr(vpnProbe),
	}, nil
}

// WANIPFallback returns an IP to use when external IP probes fail.
// Typical implementation reads the default-gateway interface's IPv4
// address from NDMS — not truly "external" if the router is behind
// CGNAT, but accurate for straight PPPoE/DHCP and better than nothing.
type WANIPFallback func(ctx context.Context) (string, error)

// GetWANIPWithFallback tries external probes first. If they all fail
// (DNS down, no internet, upstream block), it calls the fallback and
// returns whatever it produces. Errors from the fallback are surfaced;
// the original external-probe error is only returned if fallback is nil.
func GetWANIPWithFallback(ctx context.Context, fallback WANIPFallback) (string, error) {
	probe, err := fetchIPProbe(ctx, "", "")
	if err == nil {
		return probe.IP, nil
	}
	if fallback == nil {
		return "", err
	}
	fip, ferr := fallback(ctx)
	if ferr == nil && fip != "" {
		return fip, nil
	}
	return "", err
}

// fetchIPProbe fetches IP/geo using a specific service or falls back through the default list.
func fetchIPProbe(ctx context.Context, serviceURL string, iface string) (IPGeoInfo, error) {
	if serviceURL != "" {
		return fetchIPProbeOne(ctx, serviceURL, iface)
	}

	if probe, err := fetchIPProbeOne(ctx, defaultGeoIPCheckService.URL, iface); err == nil {
		return probe, nil
	}

	var lastErr error
	for _, svc := range defaultIPCheckServices {
		ip, err := fetchIPProbeOne(ctx, svc.URL, iface)
		if err != nil {
			lastErr = err
			continue
		}
		return ip, nil
	}

	if lastErr != nil {
		return IPGeoInfo{}, lastErr
	}
	return IPGeoInfo{}, fmt.Errorf("all IP services failed")
}

func CheckIPByProxy(ctx context.Context, proxyURL string, serviceURL string) (*IPGeoInfo, error) {
	probe, err := fetchIPProbeByProxy(ctx, proxyURL, serviceURL)
	if err != nil {
		return nil, err
	}
	return geoInfoPtr(probe), nil
}

func fetchIPProbeByProxy(ctx context.Context, proxyURL string, serviceURL string) (IPGeoInfo, error) {
	if serviceURL != "" {
		return fetchIPProbeOneByProxy(ctx, proxyURL, serviceURL)
	}
	if probe, err := fetchIPProbeOneByProxy(ctx, proxyURL, defaultGeoIPCheckService.URL); err == nil {
		return probe, nil
	}

	var lastErr error
	for _, svc := range defaultIPCheckServices {
		probe, err := fetchIPProbeOneByProxy(ctx, proxyURL, svc.URL)
		if err != nil {
			lastErr = err
			continue
		}
		return probe, nil
	}
	if lastErr != nil {
		return IPGeoInfo{}, lastErr
	}
	return IPGeoInfo{}, fmt.Errorf("all proxy IP services failed")
}

func fetchIPProbeOneByProxy(ctx context.Context, proxyURL string, serviceURL string) (IPGeoInfo, error) {
	res, err := httpclient.DefaultClient.Do(ctx, httpclient.CallConfig{
		URL:      serviceURL,
		ProxyURL: proxyURL,
		MaxTime:  perServiceTimeout,
	})
	if err != nil {
		return IPGeoInfo{}, fmt.Errorf("%s: %w", serviceURL, err)
	}
	info, err := parseIPProbeBody(res.Body, serviceURL)
	if err != nil {
		return IPGeoInfo{}, fmt.Errorf("%s: %w", serviceURL, err)
	}
	return info, nil
}

func lookupEndpointGeo(ctx context.Context, endpointIP string) *IPGeoInfo {
	if !isValidIP(strings.TrimSpace(endpointIP)) {
		return nil
	}
	lookupURL := fmt.Sprintf(endpointGeoLookupTemplate, neturl.PathEscape(endpointIP))
	res, err := httpclient.DefaultClient.Do(ctx, httpclient.CallConfig{
		URL:     lookupURL,
		MaxTime: perServiceTimeout,
	})
	if err != nil {
		return nil
	}

	var payload endpointGeoResponse
	if err := json.Unmarshal([]byte(res.Body), &payload); err != nil {
		if info, err2 := parseGenericIPJSON(res.Body); err2 == nil {
			if info.IP == "" {
				info.IP = endpointIP
			}
			info.Source = lookupURL
			return geoInfoPtr(info)
		}
		return nil
	}
	if strings.EqualFold(payload.Status, "fail") {
		return nil
	}
	info := IPGeoInfo{
		IP:          strings.TrimSpace(payload.Query),
		City:        strings.TrimSpace(payload.City),
		Region:      strings.TrimSpace(payload.RegionName),
		Country:     strings.TrimSpace(payload.Country),
		CountryCode: strings.TrimSpace(payload.CountryCode),
		ISP:         strings.TrimSpace(payload.ISP),
		Source:      lookupURL,
	}
	if info.IP == "" {
		info.IP = endpointIP
	}
	return geoInfoPtr(info)
}
