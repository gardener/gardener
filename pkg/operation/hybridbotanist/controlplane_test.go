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

package hybridbotanist_test

import (
	. "github.com/gardener/gardener/pkg/operation/hybridbotanist"
	"k8s.io/apimachinery/pkg/runtime/schema"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("controlplane", func() {
	Context("Shoot", func() {

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
	})
})
