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

// TODO(Rado): Do we need this to check if the received log count is correct?
// TODO(Rado): Current Vali query is 'query=count_over_time({<key>=\"<value>\"}[1h])' which returns the count of logs in the last hour.
// We might want to change it.
func ItShouldWaitForLogsWithLabelToBeInVali(s *ShootContext, valiLabels map[string]string, key, value string) {
	GinkgoHelper()

	It("Wait for logs with label to appear in Vali", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, "shoot--local--e2e-observ", key, value, s.SeedClientSet)
			if err != nil {
				return err
			}

			logCount, err := logging.GetLogCountFromSearchResponse(searchResponse)
			if err != nil {
				return err
			}

			if logCount == 0 {
				return fmt.Errorf("no logs found for %s=%s", key, value)
			}

			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Second*30))
}

func ItShouldWaitForLogsWithLabelToNotBeInVali(s *ShootContext, valiLabels map[string]string, key, value string) {
	GinkgoHelper()

	It("Ensure logs do not exist", func(ctx SpecContext) {
		// No easy way that something won't be eventually in Vali
		time.Sleep(10 * time.Second)

		Eventually(ctx, func() error {
			searchResponse, err := logging.GetValiLogs(ctx, valiLabels, "shoot--local--e2e-observ", key, value, s.SeedClientSet)
			if err != nil {
				return err
			}

			logCount, err := logging.GetLogCountFromSearchResponse(searchResponse)
			if err != nil {
				return err
			}

			if logCount > 0 {
				return fmt.Errorf("found logs in Vali for %s=%s when they were unexpected", key, value)
			}

			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Second*30))
}
