// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	. "github.com/gardener/gardener/pkg/scheduler/apis/config/helper"
	"github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var _ = Describe("Helpers test", func() {

	Describe("#ConvertSchedulerConfiguration", func() {
		externalConfiguration := v1alpha1.SchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "SchedulerConfiguration",
			},
		}

		It("should convert the external SchedulerConfiguration to an internal one", func() {
			result, err := ConvertSchedulerConfiguration(&externalConfiguration)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&config.SchedulerConfiguration{}))
		})
	})

	Describe("#ConvertSchedulerConfigurationExternal", func() {
		internalConfiguration := v1alpha1.SchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "SchedulerConfiguration",
			},
		}

		It("should convert the internal SchedulerConfiguration to an external one", func() {
			result, err := ConvertSchedulerConfigurationExternal(&internalConfiguration)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&v1alpha1.SchedulerConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       "SchedulerConfiguration",
				},
			}))
		})
	})
})
