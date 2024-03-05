// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	ExpectWithOffset(1, os.WriteFile(filepath, data, 0644)).To(Succeed())
	defer func() {
		ExpectWithOffset(1, os.Remove(filepath)).To(Succeed())
	}()

	var errBuf bytes.Buffer
	cmd := exec.Command("promtool", "test", "rules", filenameRulesTest)
	cmd.Stderr = &errBuf
	ExpectWithOffset(1, cmd.Run()).To(Succeed(), errBuf.String())
}
