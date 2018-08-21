package kubernetes

import (
	"golang.org/x/sys/unix"
	"net"
	"time"
)

func ff(tcp *net.TCPConn, timeout time.Duration) error {
	fd, err := tcp.File()
	if err != nil {
		return err
	}
	defer fd.Close()
	return unix.SetsockoptInt(int(fd.Fd()), unix.IPPROTO_TCP, unix.TCP_RXT_CONNDROPTIME, int(timeout/time.Second))
}
