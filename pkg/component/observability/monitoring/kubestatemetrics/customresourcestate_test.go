// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubestatemetrics_test

import (
	"os"
	"path/filepath"

	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.yaml.in/yaml/v2"
	"k8s.io/kube-state-metrics/v2/pkg/customresourcestate"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
)

// Returns the expected CustomResourceState config and also asserts that the actual value is the same.
// This assertion is performed inside this function to allow to give more human readable errors when the
// long config document actually differs. This also allows to keep the expectation in a standalone yaml
// file and to easily update it when it needs to be changed
func expectedCustomResourceStateConfig(suffix string) string {
	defer GinkgoRecover()

	options := []Option{WithVPAMetrics}
	relativePaths := []string{"testdata/custom-resource-state-vpa.expectation.yaml"}
	if suffix == SuffixRuntime {
		options = append(options, WithGardenResourceMetrics, WithOperatorExtensionMetrics)
		relativePaths = append(relativePaths, "testdata/custom-resource-state-garden.expectation.yaml", "testdata/custom-resource-state-operator-extension.expectation.yaml")
	}

	var expectedMetrics customresourcestate.Metrics
	// merge expected metric yamls together
	for _, path := range relativePaths {
		expectFilePath, err := filepath.Abs(path)
		Expect(err).ToNot(HaveOccurred())
		raw, err := os.ReadFile(expectFilePath)
		Expect(err).ToNot(HaveOccurred())
		// this will merge
		var m customresourcestate.Metrics
		Expect(yaml.Unmarshal(raw, &m)).ToNot(HaveOccurred())
		expectedMetrics.Spec.Resources = append(expectedMetrics.Spec.Resources, m.Spec.Resources...)
	}

	actualMetrics := NewCustomResourceStateConfig(options...)
	Expect(actualMetrics).To(BeComparableTo(expectedMetrics, cmpopts.EquateEmpty()))

	rawActual, err := yaml.Marshal(actualMetrics)
	Expect(err).ToNot(HaveOccurred())
	return string(rawActual)
}
