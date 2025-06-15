// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// SingleKnownHost is a simple HostKeyCallback that stores the host key in memory on the first callback.
// Future callbacks verify that the host key hasn't changed.
func SingleKnownHost() ssh.HostKeyCallback {
	var knownKey []byte
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if len(knownKey) == 0 {
			knownKey = key.Marshal()
			return nil
		}

		if !bytes.Equal(knownKey, key.Marshal()) {
			return fmt.Errorf("known host key does not match")
		}
		return nil
	}
}
