// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
)

var _ = Describe("controlplane", func() {
	Describe("#ValidateAuditPolicyApiGroupVersionKind", func() {
		var (
			kind = "Policy"
		)

		It("should return false without error because of version incompatibility", func() {
			incompatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range incompatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeFalse())
				}
			}
		})

		It("should return true without error because of version compatibility", func() {
			compatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.12.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.13.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.14.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.15.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range compatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
				}
			}
		})

		It("should return false with error because of not valid semver version", func() {
			shootVersion := "1.ab.0"
			gvk := auditv1.SchemeGroupVersion.WithKind(kind)

			ok, err := IsValidAuditPolicyVersion(shootVersion, &gvk)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	DescribeTable("#getResourcesForAPIServer",
		func(nodes int, storageClass, expectedCPURequest, expectedMemoryRequest, expectedCPULimit, expectedMemoryLimit string) {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit := getResourcesForAPIServer(int32(nodes), storageClass)

			Expect(cpuRequest).To(Equal(expectedCPURequest))
			Expect(memoryRequest).To(Equal(expectedMemoryRequest))
			Expect(cpuLimit).To(Equal(expectedCPULimit))
			Expect(memoryLimit).To(Equal(expectedMemoryLimit))
		},

		// nodes tests
		Entry("nodes <= 2", 2, "", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 10", 10, "", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50", 50, "", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 100", 100, "", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes > 100", 1000, "", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class tests
		Entry("scaling class small", -1, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("scaling class medium", -1, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("scaling class large", -1, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("scaling class xlarge", -1, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("scaling class 2xlarge", -1, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class always decides if provided
		Entry("nodes > 100, scaling class small", 100, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 100, scaling class medium", 100, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50, scaling class large", 50, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 10, scaling class xlarge", 10, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes <= 2, scaling class 2xlarge", 2, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),
	)

})
