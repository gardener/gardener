// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package signals

import "os"

// Info returns the OS signals used for status dumping. Fallback for non-configured, e.g., Windows, which has no equivalent signal.
func Info() []os.Signal {
	return nil
}
