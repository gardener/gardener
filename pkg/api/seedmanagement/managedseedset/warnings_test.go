// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	. "github.com/gardener/gardener/pkg/api/seedmanagement/managedseedset"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
)

var _ = Describe("Warnings", func() {
	Describe("#GetWarnings", func() {
		var managedSeedSet *seedmanagement.ManagedSeedSet

		BeforeEach(func() {
			managedSeedSet = &seedmanagement.ManagedSeedSet{}
		})

		DescribeTable("spec.shootTemplate.spec.kubernetes.kubeAPIServer",
			func(kubeAPIServerConfig *core.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when enableAnonymousAuthentication is nil",
				&core.KubeAPIServerConfig{EnableAnonymousAuthentication: nil},
				BeEmpty(),
			),
			Entry("should return a warning when enableAnonymousAuthentication is set",
				&core.KubeAPIServerConfig{EnableAnonymousAuthentication: new(true)},
				ContainElement(Equal("you are setting the spec.shootTemplate.spec.kubernetes.kubeAPIServer.enableAnonymousAuthentication field. The field is deprecated. Please use anonymous authentication configuration instead. See: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration")),
			),
		)

		DescribeTable("spec.shootTemplate.spec.kubernetes.kubeAPIServer.watchCacheSizes.default",
			func(kubeAPIServerConfig *core.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when watchCacheSizes is nil",
				&core.KubeAPIServerConfig{WatchCacheSizes: nil},
				BeEmpty(),
			),
			Entry("should not return a warning when watchCacheSizes.default is nil",
				&core.KubeAPIServerConfig{WatchCacheSizes: &core.WatchCacheSizes{Default: nil}},
				BeEmpty(),
			),
			Entry("should return a warning when watchCacheSizes.default is set",
				&core.KubeAPIServerConfig{WatchCacheSizes: &core.WatchCacheSizes{Default: new(int32(50))}},
				ContainElement(Equal("you are setting the spec.shootTemplate.spec.kubernetes.kubeAPIServer.watchCacheSizes.default field. The field has been deprecated and is forbidden to be set starting from Kubernetes 1.35. The cache size is automatically sized by the kube-apiserver.")),
			),
		)

		DescribeTable("spec.dns.providers[].secretName",
			func(dns *core.DNS, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.DNS = dns
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},
			Entry("should not return a warning when dns is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when dns.providers is nil",
				&core.DNS{Providers: nil},
				BeEmpty(),
			),
			Entry("should not return a warning when secretName is not set",
				&core.DNS{Providers: []core.DNSProvider{{SecretName: nil}}},
				BeEmpty(),
			),
			Entry("should return a warning when secretName is set",
				&core.DNS{Providers: []core.DNSProvider{{SecretName: new("secret")}}},
				ContainElement(Equal("you are setting the spec.shootTemplate.spec.dns.providers[0].secretName field. The field is deprecated and is forbidden to be set starting from Kubernetes 1.35. Use spec.shootTemplate.spec.dns.providers[0].credentialsRef instead.")),
			),
		)

		DescribeTable("spec.shootTemplate.spec.provider.workers",
			func(kubernetesVersion string, workersCount int, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.Kubernetes.Version = kubernetesVersion
				managedSeedSet.Spec.ShootTemplate.Spec.Provider.Workers = make([]core.Worker, workersCount)
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should not return a warning when the worker pools count is at the maximum", "1.37.0", 300, BeEmpty()),
			Entry("should return a warning when the worker pools count is above the maximum for Kubernetes version < 1.38", "1.37.0", 301,
				ConsistOf(Equal("you are setting the spec.shootTemplate.spec.provider.workers field to an invalid value. Invalid value: 301 worker pools, valid values: at most 300 worker pools. Invalid values will no longer be allowed for Kubernetes versions >= 1.38. See: https://github.com/gardener/gardener/issues/16107")),
			),
			Entry("should not return a warning when the worker pools count is above the maximum for Kubernetes version >= 1.38 (forbidden by validation)", "1.38.0", 301, BeEmpty()),
		)

		DescribeTable("spec.shootTemplate.spec.provider.workers[].machineControllerManagerSettings.autoPreserveFailedMachineMax",
			func(zones []string, autoPreserveFailedMachineMax int32, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.Provider.Workers = []core.Worker{{
					Name:  "test",
					Zones: zones,
					MachineControllerManagerSettings: &core.MachineControllerManagerSettings{
						AutoPreserveFailedMachineMax: &autoPreserveFailedMachineMax,
					},
				}}
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should return a warning when autoPreserveFailedMachineMax is less than the number of zones",
				[]string{"zone1", "zone2", "zone3"}, int32(2),
				ConsistOf(Equal(`you are setting the spec.shootTemplate.spec.provider.workers[0].machineControllerManagerSettings.autoPreserveFailedMachineMax field to a value (2) less than the number of zones for worker "test" (3) . This could cause some zones to have an effective autoPreserveFailedMachineMax of 0.`)),
			),
			Entry("should not return a warning when autoPreserveFailedMachineMax equals the number of zones",
				[]string{"zone1", "zone2"}, int32(2),
				BeEmpty(),
			),
			Entry("should not return a warning when autoPreserveFailedMachineMax is greater than the number of zones",
				[]string{"zone1"}, int32(2),
				BeEmpty(),
			),
			Entry("should not return a warning when autoPreserveFailedMachineMax is zero",
				[]string{"zone1", "zone2", "zone3"}, int32(0),
				BeEmpty(),
			),
		)
	})
})
