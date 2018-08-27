// +build !linux,!darwin

package kubernetes

import (
	"net"
	"time"
)

func ff(tcp *net.TCPConn, timeout time.Duration) error {
	return errUnsupported
}
