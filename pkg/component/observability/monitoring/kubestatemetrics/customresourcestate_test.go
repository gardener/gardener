// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubestatemetrics_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	"github.com/gardener/gardener/third_party/gopkg.in/yaml.v2"
)

// Returns the expected CustomResourceState config and also asserts that the actual value is the same.
// This assertion is performed inside this function to allow to give more human readable errors when the
// long config document actually differs. This also allows to keep the expectation in a standalone yaml
// file and to easily update it when it needs to be changed
func expectedCustomResourceStateConfig(suffix string) string {
	defer GinkgoRecover()
	var (
		rawActual                    []byte
		expectFilePath, relativePath string
		err                          error
		options                      []Option
	)

	options = []Option{WithVPAMetrics}
	relativePath = "testdata/custom-resource-state-vpa.expectation.yaml"

	if suffix == SuffixRuntime {
		options = append(options, WithGardenResourceMetrics)
		relativePath = "testdata/custom-resource-state-garden.expectation.yaml"
	}

	expectFilePath, err = filepath.Abs(relativePath)
	Expect(err).ToNot(HaveOccurred())
	rawActual, err = yaml.Marshal(NewCustomResourceStateConfig(options...))
	Expect(err).ToNot(HaveOccurred())

	actual := string(rawActual)
	rawExpect, err := os.ReadFile(expectFilePath)
	Expect(err).ToNot(HaveOccurred())

	if actual != string(rawExpect) {
		actualFilePath := os.TempDir() + "/custom-resource-state.actual.yaml"
		err = os.WriteFile(actualFilePath, rawActual, 0644)
		Expect(err).ToNot(HaveOccurred())

		AbortSuite("CustomResourceState configuration did not match the expectation:\n" +
			"Expected file\n" +
			"\t" + actualFilePath + "\n" +
			"to match contents from file\n" +
			"\t" + expectFilePath + "\n" +
			"Execute 'diff -Bb " + actualFilePath + " " + expectFilePath + "' to see the difference",
		)
	}

	return actual
}
