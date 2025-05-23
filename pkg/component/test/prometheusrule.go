// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"sigs.k8s.io/yaml"
)

// PrometheusRule calls the `promtool test rules` for the given PrometheusRule and the test file.
func PrometheusRule(rule *monitoringv1.PrometheusRule, filenameRulesTest string) {
	data, err := yaml.Marshal(rule.Spec.Groups)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	data = append([]byte(`groups:
`), data...)

	filepath := filepath.Join("testdata", rule.Name+".prometheusrule.yaml")
	ExpectWithOffset(1, os.WriteFile(filepath, data, 0600)).To(Succeed())
	defer func() {
		ExpectWithOffset(1, os.Remove(filepath)).To(Succeed())
	}()

	var errBuf bytes.Buffer
	cmd := exec.Command("promtool", "test", "rules", filenameRulesTest)
	cmd.Stderr = &errBuf
	ExpectWithOffset(1, cmd.Run()).To(Succeed(), errBuf.String())
}
