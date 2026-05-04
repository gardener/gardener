// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package flow

import (
	"os"
	"syscall"
)

// infoSignals returns the signals used for status dumping on macOS
func infoSignals() []os.Signal {
	return []os.Signal{syscall.SIGINFO}
}
