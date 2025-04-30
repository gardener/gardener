// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/test/e2e/gardener"

	"github.com/gardener/gardener/test/utils/shoots/logging"
)

func ItShouldWaitForLogsCountWithLabelToBeInVali(s *ShootContext, valiLabels map[string]string, key, value string, count int) {
	GinkgoHelper()

	It("Wait for logs with label to appear in Vali", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, s.ControlPlaneNamespace, key, value, s.SeedClientSet)
			if err != nil {
				return err
			}

			logCount := logging.GetLogCountFromSearchResponse(searchResponse)

			if logCount != count {
				return fmt.Errorf("expected %d logs in Vali for %s=%s, but got %d", count, key, value, logCount)
			}

			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

func ItShouldWaitForLogsWithLabelToBeInVali(s *ShootContext, valiLabels map[string]string, key, value string) {
	GinkgoHelper()

	It("Wait for logs with label to appear in Vali", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, s.ControlPlaneNamespace, key, value, s.SeedClientSet)
			if err != nil {
				return err
			}

			logCount := logging.GetLogCountFromSearchResponse(searchResponse)

			if logCount == 0 {
				return fmt.Errorf("no logs in Vali for %s=%s", key, value)
			}

			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// func ItShouldWaitForLogsWithLabelToNotBeInVali(s *ShootContext, valiLabels map[string]string, key, value string) {
// 	GinkgoHelper()
//
// 	It("Ensure logs do not exist", func(ctx SpecContext) {
// 		// No easy way to guarantee that a log won't eventually be in Vali except waiting.
// 		time.Sleep(10 * time.Second)
//
// 		Eventually(ctx, func() error {
// 			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, s.ControlPlaneNamespace, key, value, s.SeedClientSet)
// 			if err != nil {
// 				return err
// 			}
//
// 			logCount := logging.GetLogCountFromSearchResponse(searchResponse)
//
// 			if logCount > 0 {
// 				return fmt.Errorf("found logs in Vali for %s=%s when they were unexpected", key, value)
// 			}
//
// 			return nil
// 		}).Should(Succeed())
// 	}, SpecTimeout(time.Minute))
// }
