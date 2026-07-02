// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerdeployment/reference"
)

var _ = Describe("Add", func() {
	Describe("#Predicate", func() {
		var controllerDeployment *gardencorev1.ControllerDeployment

		BeforeEach(func() {
			controllerDeployment = &gardencorev1.ControllerDeployment{}
		})

		It("should return false because new object is no controllerdeployment", func() {
			Expect(Predicate(nil, nil)).To(BeFalse())
		})

		It("should return false because old object is no controllerdeployment", func() {
			Expect(Predicate(nil, controllerDeployment)).To(BeFalse())
		})

		It("should return false because there is no ref change", func() {
			Expect(Predicate(controllerDeployment, controllerDeployment)).To(BeFalse())
		})

		It("should return true because the resources field changed", func() {
			oldControllerDeployment := controllerDeployment.DeepCopy()
			controllerDeployment.Resources = []gardencorev1.NamedResourceReference{{
				Name: "resource-1",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       "test",
				},
			}}
			Expect(Predicate(oldControllerDeployment, controllerDeployment)).To(BeTrue())
		})
	})
})
