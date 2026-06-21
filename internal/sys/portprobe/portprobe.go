package portprobe

import (
	"fmt"
	"net"
)

// LocalTCPUDPFree reports whether both TCP and UDP on host:port can be bound.
// We probe both protocols because local listen-port collisions can happen in
// either namespace and sing-box mixed inbounds may need both.
func LocalTCPUDPFree(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)

	tcp, err := net.Listen("tcp4", addr)
	if err != nil {
		return false
	}
	_ = tcp.Close()

	udp, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return false
	}
	_ = udp.Close()

	return true
}
