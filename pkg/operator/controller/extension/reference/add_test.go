// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/reference"
)

var _ = Describe("Add", func() {
	Describe("#Predicate", func() {
		var extension *operatorv1alpha1.Extension

		BeforeEach(func() {
			extension = &operatorv1alpha1.Extension{
				Spec: operatorv1alpha1.ExtensionSpec{
					Deployment: &operatorv1alpha1.Deployment{},
				},
			}
		})

		It("should return false because new object is no extension", func() {
			Expect(Predicate(nil, nil)).To(BeFalse())
		})

		It("should return false because old object is no extension", func() {
			Expect(Predicate(nil, extension)).To(BeFalse())
		})

		It("should return false because there is no ref change", func() {
			Expect(Predicate(extension, extension)).To(BeFalse())
		})

		It("should return true because the resources field changed", func() {
			oldExtension := extension.DeepCopy()
			extension.Spec.Deployment.Resources = []gardencorev1.NamedResourceReference{{
				Name: "resource-1",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       "test",
				},
			}}
			Expect(Predicate(oldExtension, extension)).To(BeTrue())
		})
	})
})
