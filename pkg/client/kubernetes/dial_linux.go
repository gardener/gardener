package kubernetes

import (
	"net"
	"time"

	"golang.org/x/sys/unix"
)

const tcpUserTimeout = 0x12

func ff(tcp *net.TCPConn, timeout time.Duration) error {
	fd, err := tcp.File()
	if err != nil {
		return err
	}
	defer fd.Close()

	return unix.SetsockoptInt(int(fd.Fd()), unix.IPPROTO_TCP, tcpUserTimeout, int(timeout/time.Millisecond))
}
