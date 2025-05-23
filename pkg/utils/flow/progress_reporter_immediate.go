// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
)

type progressReporterImmediate struct {
	reporterFn ProgressReporterFn
}

// NewImmediateProgressReporter returns a new progress reporter with the given function.
func NewImmediateProgressReporter(reporterFn ProgressReporterFn) ProgressReporter {
	return progressReporterImmediate{
		reporterFn: reporterFn,
	}
}

func (p progressReporterImmediate) Start(context.Context) error { return nil }
func (p progressReporterImmediate) Stop()                       {}
func (p progressReporterImmediate) Report(ctx context.Context, stats *Stats) {
	p.reporterFn(ctx, stats)
}
