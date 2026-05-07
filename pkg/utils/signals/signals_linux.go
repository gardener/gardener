// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package signals

import (
	"os"
	"syscall"
)

// Info returns the OS signals used for status dumping (SIGUSR1 on Linux).
func Info() []os.Signal {
	return []os.Signal{syscall.SIGUSR1}
}
