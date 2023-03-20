// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		Entry("source label should be overriden by mandatory label", "test3", map[string]string{
			"app": "test3",
		}, map[string]string{
			"app":                 "istio-ingressgateway",
			"gardener.cloud/role": "exposureclass-handler",
			"handler.exposureclass.gardener.cloud/name": "test3",
		}),
	)
})
