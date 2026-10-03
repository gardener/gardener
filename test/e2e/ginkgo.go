// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
)

// These decorators are used to prioritize long-running tests cases ("boulders first, then sand"),
// see https://onsi.github.io/ginkgo/#prioritizing-specs.
const (
	// PriorityLonger is used for the longest-running tests (typically > 30 minutes).
	PriorityLonger = SpecPriority(10)
	// PriorityLong is used for long-running tests (typically > 15 minutes).
	PriorityLong = SpecPriority(5)
	// PriorityFast is used for fast tests (typically < 3 minutes).
	PriorityFast = SpecPriority(-1)
)

// CustomJUnitReport registers a ReportAfterSuite node that writes a JUnit XML report to $ARTIFACTS/junit.xml.
// Specs that were interrupted by another parallel Ginkgo process due to --fail-fast are reported as skipped instead of
// errored, so that only the actual failure (in the other process) is visible in the report.
// Call this function via a top-level var in your suite file:
//
//	var _ = e2e.CustomJUnitReport()
func CustomJUnitReport() bool {
	return ReportAfterSuite("Custom JUnit report", func(report Report) {
		artifactsDir := os.Getenv("ARTIFACTS")
		if artifactsDir == "" {
			return
		}

		for i, sr := range report.SpecReports {
			if sr.State == types.SpecStateInterrupted &&
				strings.Contains(sr.Failure.Message, "Interrupted by Other Ginkgo Process") {
				report.SpecReports[i].State = types.SpecStateSkipped
			}
		}

		Expect(reporters.GenerateJUnitReport(report, filepath.Join(artifactsDir, "junit.xml"))).To(Succeed())
	})
}
