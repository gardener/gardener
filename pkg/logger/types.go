// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logger

const (
	// DebugLevel is the debug log level, i.e. the most verbose.
	DebugLevel = "debug"
	// InfoLevel is the default log level.
	InfoLevel = "info"
	// ErrorLevel is a log level where only errors are logged.
	ErrorLevel = "error"

	// FormatJSON is the output type that produces a JSON object per log line.
	FormatJSON = "json"
	// FormatText outputs the log as human-readable text.
	FormatText = "text"
)

var (
	// AllLogLevels is a slice of all available log levels.
	AllLogLevels = []string{DebugLevel, InfoLevel, ErrorLevel}
	// AllLogFormats is a slice of all available log formats.
	AllLogFormats = []string{FormatJSON, FormatText}
)
