// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"errors"
	"os"
	"time"

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
		// Simulate a retry report via the context-based mechanism (as the flow engine would do).
		retryCtx := withRetryReporter(ctx, "task_running_1", reporter)
		ReportRetry(retryCtx, errors.New("connection timeout"))

		debugChan <- os.Interrupt

		Eventually(outBuf).Should(gbytes.Say("Flow: Test-Flow"))
		Eventually(outBuf).Should(gbytes.Say("Succeeded: 2"))
		Eventually(outBuf).Should(gbytes.Say("Failed: 1"))

		// Verify Running tasks section
		Eventually(outBuf).Should(gbytes.Say("Running Tasks:"))
		Eventually(outBuf).Should(gbytes.Say("== task_running_1"))

		// Verify Error reporting
		Eventually(outBuf).Should(gbytes.Say("Last Error:"))
		Eventually(outBuf).Should(gbytes.Say("connection timeout"))
	})

	It("should capture retry errors end-to-end when running a flow with RetryUntilTimeout", func() {
		attempts := 0
		retryErr := errors.New("transient failure")

		g := NewGraph("retry-test")
		g.Add(Task{
			Name: "flaky-task",
			Fn: TaskFn(func(_ context.Context) error {
				attempts++
				if attempts < 3 {
					return retryErr
				}
				return nil
			}).RetryUntilTimeout(10*time.Millisecond, 5*time.Second),
		})

		Expect(reporter.Start(ctx)).To(Succeed())

		err := g.Compile().Run(ctx, Opts{ProgressReporter: reporter})
		Expect(err).NotTo(HaveOccurred())
		Expect(attempts).To(Equal(3))

		// The reporter should have captured the last retry error.
		reporter.lock.Lock()
		defer reporter.lock.Unlock()
		Expect(reporter.lastErrs[TaskID("flaky-task")]).To(MatchError(retryErr))
	})
})

func newTaskIDs(ids ...string) TaskIDs {
	t := make(TaskIDs)
	for _, id := range ids {
		t.Insert(TaskID(id))
	}
	return t
}
