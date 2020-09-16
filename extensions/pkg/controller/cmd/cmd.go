// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// Log is log.Log. Exposed for testing.
	Log = log.Log
	// Exit calls os.Exit. Exposed for testing.
	Exit = os.Exit
)

// LogErrAndExit logs the given error with msg and keysAndValues and calls `os.Exit(1)`.
func LogErrAndExit(err error, msg string, keysAndValues ...interface{}) {
	Log.Error(err, msg, keysAndValues...)
	Exit(1)
}
