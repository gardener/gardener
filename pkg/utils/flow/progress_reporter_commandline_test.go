// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CommandLineProgressReporter", func() {
	var (
		reporter  *progressReporterCommandline
		outBuf    *gbytes.Buffer
		ctx       context.Context
		debugChan chan os.Signal
		testStats *Stats
	)

	BeforeEach(func() {
		outBuf = gbytes.NewBuffer()
		debugChan = make(chan os.Signal, 1)
		ctx = context.Background()

		reporter = &progressReporterCommandline{
			out:        outBuf,
			lastErrs:   make(map[TaskID]error),
			signalChan: debugChan,
			template:   parseTemplate(),
		}

		testStats = &Stats{
			FlowName:  "Test-Flow",
			Succeeded: newTaskIDs("task_success_1", "task_success_2"),
			Failed:    newTaskIDs("task_fail_1"),
			Pending:   newTaskIDs("task_pending_1"),
			Running:   newTaskIDs("task_running_1"),
		}
	})

	AfterEach(func() {
		if reporter != nil {
			reporter.Stop()
		}
	})

	It("should write the template to output when the injected signal is received", func() {
		Expect(reporter.Start(ctx)).To(Succeed())

		reporter.Report(ctx, testStats)
		reporter.ReportRetry(ctx, "task_running_1", errors.New("connection timeout"))

		debugChan <- os.Interrupt

		Eventually(outBuf).Should(And(
			gbytes.Say("Flow: Test-Flow"),
			gbytes.Say("Succeeded: 2"),
			gbytes.Say("Failed: 1"),
		))

		// Verify Running tasks section
		Eventually(outBuf).Should(And(
			gbytes.Say("Running Tasks:"),
			gbytes.Say("== task_running_1"),
		))

		// Verify Error reporting
		Eventually(outBuf).Should(And(
			gbytes.Say("Last Error:"),
			gbytes.Say("connection timeout"),
		))
	})
})

func newTaskIDs(ids ...string) TaskIDs {
	t := make(TaskIDs)
	for _, id := range ids {
		t.Insert(TaskID(id))
	}
	return t
}
