// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation/v1alpha1"
)

var _ = Describe("Config defaulting", func() {
	It("should default the selector to empty label selector", func() {
		config := &Configuration{}

		expectedConfiguration := &Configuration{
			UseGKEFormula: false,
			Selector:      &metav1.LabelSelector{},
		}

		SetObjectDefaults_Configuration(config)

		Expect(config).To(Equal(expectedConfiguration))
	})
})
