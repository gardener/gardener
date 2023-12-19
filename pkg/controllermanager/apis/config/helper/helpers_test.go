// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/apis/config/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
)

var _ = Describe("Helpers test", func() {

	Describe("#ConvertControllerManagerConfiguration", func() {
		externalConfiguration := v1alpha1.ControllerManagerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControllerManagerConfiguration",
			},
		}

		It("should convert the external ControllerManagerConfiguration to an internal one", func() {
			result, err := ConvertControllerManagerConfiguration(&externalConfiguration)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&config.ControllerManagerConfiguration{}))
		})
	})

	Describe("#ConvertControllerManagerConfigurationExternal", func() {
		internalConfiguration := config.ControllerManagerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: config.SchemeGroupVersion.String(),
				Kind:       "ControllerManagerConfiguration",
			},
		}

		It("should convert the internal ControllerManagerConfiguration to an external one", func() {
			result, err := ConvertControllerManagerConfigurationExternal(&internalConfiguration)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&v1alpha1.ControllerManagerConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       "ControllerManagerConfiguration",
				},
			}))
		})
	})
})
