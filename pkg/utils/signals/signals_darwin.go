// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package signals

import (
	"os"
	"syscall"
)

// Info returns the OS signals used for status dumping (Ctrl+T on macOS).
func Info() []os.Signal {
	return []os.Signal{syscall.SIGINFO}
}
