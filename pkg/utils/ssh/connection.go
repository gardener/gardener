// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"io"
	"io/fs"

	"github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
	"k8s.io/utils/ptr"
)

var _ = io.Closer(&Connection{})

// Connection simplifies working with SSH connections for standard command execution and connection proxying.
// Use Dial to open a new Connection, and ensure to call Connection.Close() for cleanup.
type Connection struct {
	*ssh.Client

	SCP *scp.Client

	// runAsUser is an optional user to run commands as. If set, commands (Run, Copy, etc.) are executed as the configured
	// user using sudo. Note that this requires the connecting user to have sudo permissions without password prompt.
	runAsUser string

	// outputPrefix is an optional line prefix added to stdout and stderr in Run and RunWithStreams.
	// This is useful when dealing with multiple connections for marking output with different connection information.
	outputPrefix string
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

	sshClient := ssh.NewClient(conn, chans, reqs)
	return &Connection{
		Client: sshClient,
		SCP:    ptr.To(scp.NewConfigurer("", nil).SSHClient(sshClient).Create()),
	}, nil
}

// RunAsUser configures the connection to run commands (Run, Copy, etc.) as the given user using sudo.
func (c *Connection) RunAsUser(user string) *Connection {
	c.runAsUser = user

	if c.runAsUser != "" {
		c.SCP.RemoteBinary = "sudo -u " + c.runAsUser + " scp"
	} else {
		c.SCP.RemoteBinary = "scp"
	}

	return c
}

// WithOutputPrefix configures the connection to add the given line prefix to stdout and stderr in Run and RunWithStreams.
func (c *Connection) WithOutputPrefix(prefix string) *Connection {
	c.outputPrefix = prefix
	return c
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

	if c.runAsUser != "" {
		command = "sudo -u " + c.runAsUser + " " + command
	}

	session.Stdin = stdin

	session.Stdout = stdout
	if c.outputPrefix != "" {
		session.Stdout = NewPrefixedWriter(c.outputPrefix, session.Stdout)
	}
	session.Stderr = stderr
	if c.outputPrefix != "" {
		session.Stderr = NewPrefixedWriter(c.outputPrefix, session.Stderr)
	}

	return session.Run(command)
}

// Copy copies the given bytes to a file at remotePath with the given permissions.
func (c *Connection) Copy(ctx context.Context, remotePath, permissions string, data []byte) error {
	return c.SCP.Copy(ctx, bytes.NewReader(data), remotePath, permissions, int64(len(data)))
}

// CopyFile copies the file to remotePath with the given permissions.
func (c *Connection) CopyFile(ctx context.Context, remotePath, permissions string, file fs.File) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// we can't use CopyFromFile because it requires an os.File instead of fs.File.
	return c.SCP.Copy(ctx, file, remotePath, permissions, stat.Size())
}
