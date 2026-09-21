// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("getBootstrapConfiguration", func() {
	var (
		worker                    gardencorev1beta1.Worker
		kubeletDataVolumeName     = "kubelet-data-vol"
		kubeletDataVolumeSizeInKi = int64(1369088) // 1337Ki in bytes
	)

	BeforeEach(func() {
		worker = gardencorev1beta1.Worker{}
	})

	When("bootstrapConfiguration is nil", func() {
		It("should return an empty configuration when no kubelet data volume is configured", func() {
			result, err := getBootstrapConfiguration(nil, worker)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&nodeagentconfigv1alpha1.BootstrapConfiguration{}))
		})

		It("should return a configuration with KubeletDataVolumeSize when a kubelet data volume is configured", func() {
			worker.KubeletDataVolumeName = &kubeletDataVolumeName
			worker.DataVolumes = []gardencorev1beta1.DataVolume{{Name: kubeletDataVolumeName, VolumeSize: "1337Ki"}}

			result, err := getBootstrapConfiguration(nil, worker)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&nodeagentconfigv1alpha1.BootstrapConfiguration{
				KubeletDataVolumeSize: &kubeletDataVolumeSizeInKi,
			}))
		})
	})

	When("bootstrapConfiguration is non-nil", func() {
		It("should preserve existing fields and leave KubeletDataVolumeSize nil when no kubelet data volume is configured", func() {
			existing := &nodeagentconfigv1alpha1.BootstrapConfiguration{
				ControlPlaneNodesEndpoints: &nodeagentconfigv1alpha1.ControlPlaneNodesEndpoints{Enabled: true},
			}

			result, err := getBootstrapConfiguration(existing, worker)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&nodeagentconfigv1alpha1.BootstrapConfiguration{
				ControlPlaneNodesEndpoints: &nodeagentconfigv1alpha1.ControlPlaneNodesEndpoints{Enabled: true},
				KubeletDataVolumeSize:      nil,
			}))
		})

		It("should preserve existing fields and set KubeletDataVolumeSize when a kubelet data volume is configured", func() {
			existing := &nodeagentconfigv1alpha1.BootstrapConfiguration{
				ControlPlaneNodesEndpoints: &nodeagentconfigv1alpha1.ControlPlaneNodesEndpoints{Enabled: true},
			}
			worker.KubeletDataVolumeName = &kubeletDataVolumeName
			worker.DataVolumes = []gardencorev1beta1.DataVolume{{Name: kubeletDataVolumeName, VolumeSize: "1337Ki"}}

			result, err := getBootstrapConfiguration(existing, worker)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&nodeagentconfigv1alpha1.BootstrapConfiguration{
				ControlPlaneNodesEndpoints: &nodeagentconfigv1alpha1.ControlPlaneNodesEndpoints{Enabled: true},
				KubeletDataVolumeSize:      &kubeletDataVolumeSizeInKi,
			}))
		})
	})
})
