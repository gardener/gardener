// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
)

var _ = Describe("ViewerKubeconfigRequest defaulting", func() {
	var obj *ViewerKubeconfigRequest

	BeforeEach(func() {
		obj = &ViewerKubeconfigRequest{}
	})

	Describe("ExpirationSeconds defaulting", func() {
		It("should default expirationSeconds field", func() {
			SetObjectDefaults_ViewerKubeconfigRequest(obj)

			Expect(obj.Spec.ExpirationSeconds).To(Equal(pointer.Int64(60 * 60)))
		})

		It("should not default expirationSeconds field if it is already set", func() {
			obj.Spec.ExpirationSeconds = pointer.Int64(10 * 60)

			SetObjectDefaults_ViewerKubeconfigRequest(obj)

			Expect(obj.Spec.ExpirationSeconds).To(Equal(pointer.Int64(10 * 60)))
		})
	})
})
