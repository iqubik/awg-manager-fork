package httpclient

import (
	"context"
	"net"
	"strings"
	"time"
)

func buildDialContext(bindIface string, dnsServers []string, connectTimeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	dialer := bindDialer(bindIface, connectTimeout)
	bindIface = strings.TrimSpace(bindIface)
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if bindIface != "" {
			network = "tcp4"
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if net.ParseIP(host) == nil {
				ip, err := LookupIPv4ForBind(ctx, host, dnsServers, bindIface)
				if err != nil {
					return nil, err
				}
				addr = net.JoinHostPort(ip, port)
			}
		}
		return dialer.DialContext(ctx, network, addr)
	}
}
