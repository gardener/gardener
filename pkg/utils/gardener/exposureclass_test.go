// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("ExposureClass", func() {
	DescribeTable("#GetMandatoryExposureClassHandlerSNILabels",
		func(name string, labels, expectedLabels map[string]string) {
			Expect(GetMandatoryExposureClassHandlerSNILabels(labels, name)).To(Equal(expectedLabels))
		},

		Entry("target labels contain only mandatory labels as source labels are empty", "test1", map[string]string{}, map[string]string{
			"app":                 "istio-ingressgateway",
			"gardener.cloud/role": "exposureclass-handler",
			"handler.exposureclass.gardener.cloud/name": "test1",
		}),
		Entry("target labels contain source and mandatory labels", "test2", map[string]string{
			"gardener": "test2",
		}, map[string]string{
			"gardener":            "test2",
			"app":                 "istio-ingressgateway",
			"gardener.cloud/role": "exposureclass-handler",
			"handler.exposureclass.gardener.cloud/name": "test2",
		}),
		Entry("source label should be overridden by mandatory label", "test3", map[string]string{
			"app": "test3",
		}, map[string]string{
			"app":                 "istio-ingressgateway",
			"gardener.cloud/role": "exposureclass-handler",
			"handler.exposureclass.gardener.cloud/name": "test3",
		}),
	)
})
