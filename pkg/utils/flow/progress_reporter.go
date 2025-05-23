// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"strings"
)

// ProgressReporterFn is continuously called on progress in a flow.
type ProgressReporterFn func(context.Context, *Stats)

// ProgressReporter is used to report the current progress of a flow.
type ProgressReporter interface {
	// Start starts the progress reporter.
	Start(context.Context) error
	// Stop stops the progress reporter.
	Stop()
	// Report reports the progress using the current statistics.
	Report(context.Context, *Stats)
}

// MakeDescription returns a description based on the stats.
func MakeDescription(stats *Stats) string {
	if stats.ProgressPercent() == 0 {
		return "Starting " + stats.FlowName
	}
	if stats.ProgressPercent() == 100 {
		return stats.FlowName + " finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}
