// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
