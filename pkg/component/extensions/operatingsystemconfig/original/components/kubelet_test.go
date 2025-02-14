// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package components_test

import (
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
)

var _ = ginkgo.Describe("Kubelet", func() {
	ginkgo.Describe("CalculateDataStringForKubeletConfiguration", func() {
		var kubeletConfig *gardencorev1beta1.KubeletConfig

		ginkgo.BeforeEach(func() {
			kubeletConfig = &gardencorev1beta1.KubeletConfig{}
		})

		ginkgo.It("should return an empty string if the kubelet config is nil", func() {
			Expect(components.CalculateDataStringForKubeletConfiguration(nil)).To(BeEmpty())
		})

		ginkgo.It("should return an empty string if the kubelet config is empty", func() {
			Expect(components.CalculateDataStringForKubeletConfiguration(kubeletConfig)).To(BeEmpty())
		})

		ginkgo.It("should return the correct data string for the kubelet config", func() {
			kubeletConfig = &gardencorev1beta1.KubeletConfig{
				CPUManagerPolicy: ptr.To("static"),
				EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
					ImageFSAvailable:  ptr.To("200Mi"),
					ImageFSInodesFree: ptr.To("1k"),
					MemoryAvailable:   ptr.To("200Mi"),
					NodeFSAvailable:   ptr.To("200Mi"),
					NodeFSInodesFree:  ptr.To("1k"),
				},
				SystemReserved: &gardencorev1beta1.KubeletConfigReserved{
					CPU:              ptr.To(resource.MustParse("1m")),
					Memory:           ptr.To(resource.MustParse("1Mi")),
					PID:              ptr.To(resource.MustParse("1k")),
					EphemeralStorage: ptr.To(resource.MustParse("100Gi")),
				},
				KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
					CPU:              ptr.To(resource.MustParse("100m")),
					Memory:           ptr.To(resource.MustParse("2Gi")),
					PID:              ptr.To(resource.MustParse("15k")),
					EphemeralStorage: ptr.To(resource.MustParse("42Gi")),
				},
			}

			Expect(components.CalculateDataStringForKubeletConfiguration(kubeletConfig)).To(ConsistOf(
				"101m-2049Mi-16k-142Gi",
				"200Mi-1k-200Mi-200Mi-1k",
				"static",
			))
		})
	})
})
