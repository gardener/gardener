// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"io"

	"golang.org/x/crypto/ssh"
)

var _ = io.Closer(&Connection{})

// Connection simplifies working with SSH connections for standard command execution and connection proxying.
// Use Dial to open a new Connection, and ensure to call Connection.Close() for cleanup.
type Connection struct {
	*ssh.Client

	// OutputPrefix is an optional line prefix added to stdout and stderr in Run and RunWithStreams.
	// This is useful when dealing with multiple connections for marking output with different connection information.
	OutputPrefix string
}

// Dial opens a new SSH Connection. Ensure to call Connection.Close() for cleanup.
func Dial(ctx context.Context, addr string, opts ...Option) (*Connection, error) {
	config := DefaultConfig()
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	tcpConn, err := config.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	conn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, &config.ClientConfig)
	if err != nil {
		_ = tcpConn.Close()
		return nil, err
	}

	return &Connection{Client: ssh.NewClient(conn, chans, reqs)}, nil
}

// Run executes the given command on the remote host and returns stdout and stderr streams.
func (c *Connection) Run(command string) (io.Reader, io.Reader, error) {
	var stdout, stderr bytes.Buffer
	return &stdout, &stderr, c.RunWithStreams(nil, &stdout, &stderr, command)
}

// RunWithStreams executes the given command on the remote host with the configured streams.
func (c *Connection) RunWithStreams(stdin io.Reader, stdout, stderr io.Writer, command string) error {
	session, err := c.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = stdin

	session.Stdout = stdout
	if c.OutputPrefix != "" {
		session.Stdout = NewPrefixedWriter(c.OutputPrefix, session.Stdout)
	}
	session.Stderr = stderr
	if c.OutputPrefix != "" {
		session.Stderr = NewPrefixedWriter(c.OutputPrefix, session.Stderr)
	}

	return session.Run(command)
}
