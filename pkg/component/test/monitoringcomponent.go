// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/component"
)

// ScrapeConfigs is a utility test function for MonitoringComponents in order to test the scape configurations.
func ScrapeConfigs(c component.MonitoringComponent, expectedScrapeConfigs ...string) {
	scrapeConfigs, err := c.ScrapeConfigs()
	Expect(err).NotTo(HaveOccurred())

	matchers := make([]interface{}, 0, len(expectedScrapeConfigs))
	for _, sc := range expectedScrapeConfigs {
		matchers = append(matchers, Equal(sc))
	}

	Expect(scrapeConfigs).To(ConsistOf(matchers...))
}

// AlertingRules is a utility test function for MonitoringComponents in order to test the alerting rules.
func AlertingRules(c component.MonitoringComponent, expectedAlertingRules map[string]string) {
	alertingRules, err := c.AlertingRules()
	Expect(err).NotTo(HaveOccurred())
	Expect(alertingRules).To(HaveLen(len(expectedAlertingRules)))

	for filename, rule := range expectedAlertingRules {
		Expect(alertingRules).To(HaveKeyWithValue(filename, rule))
	}
}

// ExecutePromtool execute the promtool tests for the alerting rules. It writes the rules into a temporary file in the
// "testdata" directory.
func ExecutePromtool(c component.MonitoringComponent, filenameRulesTest string) {
	alertingRules, err := c.AlertingRules()
	Expect(err).NotTo(HaveOccurred())

	for filename, rule := range alertingRules {
		filepath := filepath.Join("testdata", filename)

		Expect(os.WriteFile(filepath, []byte(rule), 0644)).To(Succeed())

		var errBuf bytes.Buffer
		cmd := exec.Command("promtool", "test", "rules", filenameRulesTest)
		cmd.Stderr = &errBuf
		Expect(cmd.Run()).To(Succeed(), errBuf.String())

		Expect(os.Remove(filepath)).To(Succeed())
	}
}

// AlertingRulesWithPromtool tests the alerting rules and execute promtool tests.
func AlertingRulesWithPromtool(c component.MonitoringComponent, expectedAlertingRules map[string]string, filenameRulesTest string) {
	AlertingRules(c, expectedAlertingRules)
	if filenameRulesTest != "" {
		ExecutePromtool(c, filenameRulesTest)
	}
}
