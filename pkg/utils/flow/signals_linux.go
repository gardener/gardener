// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package flow

import (
	"os"
	"syscall"
)

// InfoSignals returns the signals used for status dumping on macOS
func infoSignals() []os.Signal {
	return []os.Signal{syscall.SIGUSR1}
}
