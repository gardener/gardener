// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
)

var _ = Describe("AdminKubeconfigRequest defaulting", func() {
	var obj *AdminKubeconfigRequest

	BeforeEach(func() {
		obj = &AdminKubeconfigRequest{}
	})

	Describe("ExpirationSeconds defaulting", func() {
		It("should default expirationSeconds field", func() {
			SetObjectDefaults_AdminKubeconfigRequest(obj)

			Expect(obj.Spec.ExpirationSeconds).To(PointTo(Equal(int64(60 * 60))))
		})

		It("should not default expirationSeconds field if it is already set", func() {
			obj.Spec.ExpirationSeconds = ptr.To(int64(10 * 60))

			SetObjectDefaults_AdminKubeconfigRequest(obj)

			Expect(obj.Spec.ExpirationSeconds).To(PointTo(Equal(int64(10 * 60))))
		})
	})
})
