// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
)

var _ = Describe("Changes", func() {
	DescribeTable("#CheckIfMinorVersionUpdate", func(oldVersion, newVersion string, expected bool, errMatcher gomegatypes.GomegaMatcher) {
		isMinorVersionUpdate, err := CheckIfMinorVersionUpdate(oldVersion, newVersion)
		Expect(err).To(errMatcher)
		Expect(isMinorVersionUpdate).To(Equal(expected))
	},
		Entry("invalid version", "a.b.c", "1.0.0", false, MatchError(ContainSubstring("failed to parse"))),
		Entry("1.0.0 to 1.1.0", "1.0.0", "1.1.0", true, BeNil()),
		Entry("1.1.0 to 1.1.2", "1.1.0", "1.1.2", false, BeNil()),
		Entry("v1.1.0 to v1.1.2", "v1.1.0", "v1.1.2", false, BeNil()),
		Entry("v1.2.3 to v1.3.4", "v1.2.3", "v1.3.4", true, BeNil()),
		Entry("v1.3.4 to v1.5.6", "v1.3.4", "v1.5.6", true, BeNil()),
		Entry("1.2.3-foo.12 to 1.2.4-foo.23", "1.2.3-foo.12", "1.2.4-foo.12", false, BeNil()),
		Entry("1.1.1-foo.12 to 1.2.3-foo.23", "1.1.1-foo.12", "1.2.3-foo.23", true, BeNil()),
	)

	DescribeTable("#ComputeKubeletConfigChange", func(oldKubeletConfig, newKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration, expectedCPUManagerPolicyChange, expectedConfigChange bool, errMatcher gomegatypes.GomegaMatcher) {
		configChange, cpuManagerPolicyChange, err := ComputeKubeletConfigChange(oldKubeletConfig, newKubeletConfig)
		Expect(err).To(errMatcher)
		Expect(cpuManagerPolicyChange).To(Equal(expectedCPUManagerPolicyChange))
		Expect(configChange).To(Equal(expectedConfigChange))
	},
		Entry("no change", &kubeletconfigv1beta1.KubeletConfiguration{}, &kubeletconfigv1beta1.KubeletConfiguration{}, false, false, BeNil()),
		Entry("changed cpu manager policy", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static"}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none"}, true, false, BeNil()),

		Entry("invalid kubeReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", KubeReserved: map[string]string{"cpu": "aoeu"}, SystemReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", KubeReserved: map[string]string{"cpu": "100m"}}, true, false, MatchError(ContainSubstring("failed to parse"))),
		Entry("changed kubeReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", KubeReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", KubeReserved: map[string]string{"cpu": "200m"}}, true, true, BeNil()),
		Entry("changed kubeReserved memory", &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"memory": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"memory": "200Mi"}}, false, true, BeNil()),
		Entry("changed kubeReserved ephemeral-storage", &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"ephemeral-storage": "100Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"ephemeral-storage": "200Gi"}}, false, true, BeNil()),
		Entry("changed kubeReserved PID", &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"pids": "10k"}}, &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"pids": "20k"}}, false, true, BeNil()),

		Entry("invalid systemReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", KubeReserved: map[string]string{"cpu": "100m"}, SystemReserved: map[string]string{"cpu": "aoeu"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", KubeReserved: map[string]string{"cpu": "100m"}}, true, false, MatchError(ContainSubstring("failed to parse"))),
		Entry("changed systemReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", SystemReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", SystemReserved: map[string]string{"cpu": "200m"}}, true, true, BeNil()),
		Entry("changed systemReserved memory", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "200Mi"}}, false, true, BeNil()),
		Entry("changed systemReserved ephemeral-storage", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "100Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "200Gi"}}, false, true, BeNil()),
		Entry("changed systemReserved PID", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pids": "10k"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pids": "20k"}}, false, true, BeNil()),

		Entry("sum of systemReserved and kubeReserved cpu changed", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", SystemReserved: map[string]string{"cpu": "100m"}, KubeReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", SystemReserved: map[string]string{"cpu": "200m"}, KubeReserved: map[string]string{"cpu": "200m"}}, true, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved cpu remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"cpu": "100m"}, KubeReserved: map[string]string{"cpu": "50m"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"cpu": "50m"}, KubeReserved: map[string]string{"cpu": "100m"}}, false, false, BeNil()),
		Entry("sum of systemReserved and kubeReserved memory changed", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "100Mi"}, KubeReserved: map[string]string{"memory": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "200Mi"}, KubeReserved: map[string]string{"memory": "200Mi"}}, false, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved memory remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "100Mi"}, KubeReserved: map[string]string{"memory": "50Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "50Mi"}, KubeReserved: map[string]string{"memory": "100Mi"}}, false, false, BeNil()),
		Entry("sum of systemReserved and kubeReserved ephemeral-storage changed", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "100Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "100Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "200Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "200Gi"}}, false, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved ephemeral-storage remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "100Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "50Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "50Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "100Gi"}}, false, false, BeNil()),
		Entry("sum of systemReserved and kubeReserved PID changed", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "10k"}, KubeReserved: map[string]string{"pid": "10k"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "20k"}, KubeReserved: map[string]string{"pid": "20k"}}, false, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved PID remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "10k"}, KubeReserved: map[string]string{"pid": "5k"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "5k"}, KubeReserved: map[string]string{"pid": "10k"}}, false, false, BeNil()),

		Entry("changed evictionHard memory.available", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", EvictionHard: map[string]string{"memory.available": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", EvictionHard: map[string]string{"memory.available": "200Mi"}}, true, true, BeNil()),
		Entry("changed evictionHard imagefs.available", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.available": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.available": "200Mi"}}, false, true, BeNil()),
		Entry("changed evictionHard imagefs.inodesFree", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.inodesFree": "1k"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.inodesFree": "2k"}}, false, true, BeNil()),
		Entry("changed evictionHard nodefs.available", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.available": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.available": "200Mi"}}, false, true, BeNil()),
		Entry("changed evictionHard nodefs.inodesFree", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.inodesFree": "1k"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.inodesFree": "2k"}}, false, true, BeNil()),
		Entry("some other field changed in evictionHard", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"foo": "bar"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"foo": "baz"}}, false, false, BeNil()),
	)

	Describe("#IsOsVersionUpToDate", func() {
		var (
			currentOSVersion *string
			newOSC           *extensionsv1alpha1.OperatingSystemConfig
		)
		BeforeEach(func() {
			currentOSVersion = nil
			newOSC = &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdates{
						OperatingSystemVersion: "1.2.0",
					},
				},
			}
		})

		It("should return false if current OS version is nil", func() {
			changed, err := IsOsVersionUpToDate(currentOSVersion, newOSC)
			Expect(err).To(MatchError(ContainSubstring("current OS version is nil")))
			Expect(changed).To(BeFalse())
		})

		It("should return true if the OS version is up to date", func() {
			currentOSVersion = ptr.To("1.2")
			changed, err := IsOsVersionUpToDate(currentOSVersion, newOSC)
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeTrue())

			currentOSVersion = ptr.To("1.2.0")
			changed, err = IsOsVersionUpToDate(currentOSVersion, newOSC)
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeTrue())

			currentOSVersion = ptr.To("1.2.0-foo.12")
			newOSC.Spec.InPlaceUpdates.OperatingSystemVersion = "1.2.0"
			changed, err = IsOsVersionUpToDate(currentOSVersion, newOSC)
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeTrue())
		})

		It("should return false if the OS version is not up to date", func() {
			currentOSVersion = ptr.To("1.1.0")
			changed, err := IsOsVersionUpToDate(currentOSVersion, newOSC)
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeFalse())

			currentOSVersion = ptr.To("1.2.0")
			newOSC.Spec.InPlaceUpdates.OperatingSystemVersion = "1.2.1"
			changed, err = IsOsVersionUpToDate(currentOSVersion, newOSC)
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeFalse())
		})

		It("should return an error if the OS version in the new OSC is invalid", func() {
			newOSC.Spec.InPlaceUpdates.OperatingSystemVersion = "invalid"
			currentOSVersion = ptr.To("1.2.0")
			changed, err := IsOsVersionUpToDate(currentOSVersion, newOSC)
			Expect(err).To(MatchError(ContainSubstring("failed comparing current OS version")))
			Expect(changed).To(BeFalse())
		})
	})

	Describe("ComputeCredentialsRotationChanges", func() {
		var (
			oldOSC, newOSC *extensionsv1alpha1.OperatingSystemConfig
			timeNow        = time.Now().UTC()
		)

		BeforeEach(func() {
			oldOSC = &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdates{
						CredentialsRotation: &extensionsv1alpha1.CredentialsRotation{
							CertificateAuthorities: &extensionsv1alpha1.CARotation{
								LastInitiationTime: &metav1.Time{Time: timeNow.Add(-time.Hour)},
							},
							ServiceAccountKey: &extensionsv1alpha1.ServiceAccountKeyRotation{
								LastInitiationTime: &metav1.Time{Time: timeNow.Add(-time.Hour)},
							},
						},
					},
				},
			}

			newOSC = oldOSC.DeepCopy()
			newOSC.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities.LastInitiationTime = &metav1.Time{Time: timeNow}
			newOSC.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey.LastInitiationTime = &metav1.Time{Time: timeNow}
		})

		It("should return false if CredentialsRotation is nil in the new OSC", func() {
			oldOSC.Spec.InPlaceUpdates.CredentialsRotation = nil
			newOSC.Spec.InPlaceUpdates.CredentialsRotation = nil

			caRotation, saKeyRotation := ComputeCredentialsRotationChanges(oldOSC, newOSC)
			Expect(caRotation).To(BeFalse())
			Expect(saKeyRotation).To(BeFalse())
		})

		It("should return true if the CredentialsRotation is nil in the old OSC", func() {
			oldOSC.Spec.InPlaceUpdates.CredentialsRotation = nil

			caRotation, saKeyRotation := ComputeCredentialsRotationChanges(oldOSC, newOSC)
			Expect(caRotation).To(BeTrue())
			Expect(saKeyRotation).To(BeTrue())
		})

		It("should return true if the lastInitiationTimes of rotations are changed", func() {
			caRotation, saKeyRotation := ComputeCredentialsRotationChanges(oldOSC, newOSC)
			Expect(caRotation).To(BeTrue())
			Expect(saKeyRotation).To(BeTrue())
		})

		It("should return false if the lastInitiationTimes of rotations are not changed", func() {
			oldOSC = newOSC.DeepCopy()

			caRotation, saKeyRotation := ComputeCredentialsRotationChanges(oldOSC, newOSC)
			Expect(caRotation).To(BeFalse())
			Expect(saKeyRotation).To(BeFalse())
		})
	})
})
