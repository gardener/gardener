// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/shoots/logging"
)

// ItShouldWaitForLogsCountWithLabelToBeInVali waits for a specific number of logs with a given label to be present in Vali.
func ItShouldWaitForLogsCountWithLabelToBeInVali(s *ShootContext, valiLabels map[string]string, key, value string, expectedLogCount int) {
	GinkgoHelper()

	It("Wait for logs with label to appear in Vali", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, s.ControlPlaneNamespace, key, value, s.SeedClientSet)
			if err != nil {
				return err
			}

			if logCount := logging.GetLogCountFromSearchResponse(searchResponse); logCount != expectedLogCount {
				return fmt.Errorf("expected %d logs in Vali for %s=%s, but got %d", expectedLogCount, key, value, logCount)
			}

			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForLogsWithLabelToBeInVali waits for logs with a specific label to be present in Vali. Does not regard the count of the logs.
func ItShouldWaitForLogsWithLabelToBeInVali(s *ShootContext, valiLabels map[string]string, key, value string) {
	GinkgoHelper()

	It("Wait for logs with label to appear in Vali", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, s.ControlPlaneNamespace, key, value, s.SeedClientSet)
			if err != nil {
				return err
			}

			if logging.GetLogCountFromSearchResponse(searchResponse) == 0 {
				return fmt.Errorf("no logs in Vali for %s=%s", key, value)
			}

			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForLogsWithLabelToNotBeInVali check that, after a timeout, logs with a specific label are NOT present in Vali. This check is not perfectly strict.
func ItShouldWaitForLogsWithLabelToNotBeInVali(s *ShootContext, valiLabels map[string]string, key, value string) {
	GinkgoHelper()

	// We need to ensure that logs for the pod are not found in Vali.
	// The only way we can guarantee that is to wait and then naively check Vali.
	// The `Consistently` check is used for this reason. It waits for a
	// certain period of time and check whether the condition is true on
	// every specified interval.
	It("Ensure logs do not exist", func(ctx SpecContext) {
		Consistently(ctx, func() error {
			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, s.ControlPlaneNamespace, key, value, s.SeedClientSet)
			if err != nil {
				return err
			}

			if logging.GetLogCountFromSearchResponse(searchResponse) > 0 {
				return fmt.Errorf("found logs in Vali for %s=%s when they were unexpected", key, value)
			}

			return nil
		}, 10*time.Second, 5*time.Second).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
