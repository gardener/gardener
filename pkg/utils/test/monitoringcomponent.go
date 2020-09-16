// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/gardener/gardener/pkg/operation/botanist/component"

	. "github.com/onsi/gomega"
)

// AlertingRules is a utility test function for MonitoringComponents in order to test the alerting rules with the
// promtool binary.
func AlertingRules(c component.MonitoringComponent, expectedAlertingRules map[string]string, filenameRulesTest string) {
	alertingRules, err := c.AlertingRules()
	Expect(err).NotTo(HaveOccurred())
	Expect(alertingRules).To(HaveLen(len(expectedAlertingRules)))

	for filename, rule := range expectedAlertingRules {
		Expect(alertingRules).To(HaveKeyWithValue(filename, rule))
	}

	for filename, rule := range alertingRules {
		Expect(ioutil.WriteFile(filename, []byte(rule), 0644)).To(Succeed())

		var errBuf bytes.Buffer
		cmd := exec.Command("promtool", "test", "rules", filenameRulesTest)
		cmd.Stderr = &errBuf
		Expect(cmd.Run()).To(Succeed(), errBuf.String())

		Expect(os.Remove(filename)).To(Succeed())
	}
}
