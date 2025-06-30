// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"crypto/rsa"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config contains configuration for connecting to remote hosts.
type Config struct {
	// ClientConfig is the standard SSH client config.
	ssh.ClientConfig
	// DialContext is used for opening TCP connections to the remote host. Defaults to a net.Dialer with 30s timeout.
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

// DefaultConfig is the default SSH client Config used by Dial.
func DefaultConfig() *Config {
	return &Config{
		ClientConfig: ssh.ClientConfig{
			Timeout:         30 * time.Second,
			HostKeyCallback: SingleKnownHost(),
		},
		DialContext: (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
	}
}

// Option is an option that can be passed to Dial for customizing the client Config.
type Option func(opts *Config) error

// WithUser configures the login user.
func WithUser(user string) Option {
	return func(opts *Config) error {
		opts.User = user
		return nil
	}
}

// WithPrivateKey configures the client to authenticate using the given RSA private key.
func WithPrivateKey(key *rsa.PrivateKey) Option {
	return func(opts *Config) error {
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			return err
		}
		opts.Auth = append(opts.Auth, ssh.PublicKeys(signer))
		return nil
	}
}

// WithPrivateKeyBytes configures the client to authenticate using the given PEM encoded private key.
func WithPrivateKeyBytes(key []byte) Option {
	return func(opts *Config) error {
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return err
		}
		opts.Auth = append(opts.Auth, ssh.PublicKeys(signer))
		return nil
	}
}

// WithProxyConnection configures the client to open the new TCP connection to the remote host via another open SSH
// connection.
func WithProxyConnection(conn *Connection) Option {
	return func(opts *Config) error {
		opts.DialContext = conn.DialContext
		return nil
	}
}
