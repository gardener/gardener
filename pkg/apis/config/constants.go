// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

const (
	// LogLevelDebug is the debug log level, i.e. the most verbose.
	LogLevelDebug = "debug"
	// LogLevelInfo is the default log level.
	LogLevelInfo = "info"
	// LogLevelError is a log level where only errors are logged.
	LogLevelError = "error"

	// LogFormatJSON is the output type that produces a JSON object per log line.
	LogFormatJSON = "json"
	// LogFormatText outputs the log as human-readable text.
	LogFormatText = "text"
)
