package kubernetes

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"net"
	"time"
)

var errUnsupported = errors.New("tcp fast fail is unsupported on this platform")

type dialFunc func(ctx context.Context, network, address string) (net.Conn, error)

type failFastDial struct {
	dialFunc dialFunc
	timeout  time.Duration
}

func newFailFastDial(dial dialFunc, timeout time.Duration) *failFastDial {
	return &failFastDial{dial, timeout}
}

func (f *failFastDial) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := f.dialFunc(ctx, network, address)
	if err != nil {
		return nil, err
	}

	if tcp, ok := conn.(*net.TCPConn); ok {
		tcpErr := ff(tcp, f.timeout)
		switch tcpErr {
		case nil:
			logrus.Debugf("Enabled fast failing tcp connection %s:%s. "+
				"Connections will be terminated after %v of unacknowledged transmissions",
				network, address, f.timeout)
		case errUnsupported:
			logrus.Warn("Fast failing tcp connections are not enabled on this platform. " +
				"It may take a long time (> 15min) for dead connections to really be considered dead")
		default:
			conn.Close()
			return nil, err
		}
	} else {
		logrus.Debugf("Connection is no TCP connection, skipping fail fast")
	}

	return conn, err
}
