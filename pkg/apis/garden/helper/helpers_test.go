// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	. "github.com/gardener/gardener/pkg/apis/garden/helper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("helper", func() {
	Describe("#DetermineCloudProviderInProfile", func() {
		It("should return cloud provider AWS", func() {
			spec := garden.CloudProfileSpec{
				AWS: &garden.AWSProfile{},
			}

			cloudProvider, err := DetermineCloudProviderInProfile(spec)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderAWS))
		})

		It("should return cloud provider Azure", func() {
			spec := garden.CloudProfileSpec{
				Azure: &garden.AzureProfile{},
			}

			cloudProvider, err := DetermineCloudProviderInProfile(spec)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderAzure))
		})

		It("should return cloud provider GCP", func() {
			spec := garden.CloudProfileSpec{
				GCP: &garden.GCPProfile{},
			}

			cloudProvider, err := DetermineCloudProviderInProfile(spec)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderGCP))
		})

		It("should return cloud provider OpenStack", func() {
			spec := garden.CloudProfileSpec{
				OpenStack: &garden.OpenStackProfile{},
			}

			cloudProvider, err := DetermineCloudProviderInProfile(spec)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderOpenStack))
		})

		It("should return an error because no cloud provider is set", func() {
			spec := garden.CloudProfileSpec{}

			_, err := DetermineCloudProviderInProfile(spec)

			Expect(err).To(HaveOccurred())
		})

		It("should return an error because too many cloud providers are set", func() {
			spec := garden.CloudProfileSpec{
				AWS:   &garden.AWSProfile{},
				Azure: &garden.AzureProfile{},
			}

			_, err := DetermineCloudProviderInProfile(spec)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DetermineCloudProviderInShoot", func() {
		It("should return cloud provider AWS", func() {
			cloud := garden.Cloud{
				AWS: &garden.AWSCloud{},
			}

			cloudProvider, err := DetermineCloudProviderInShoot(cloud)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderAWS))
		})

		It("should return cloud provider Azure", func() {
			cloud := garden.Cloud{
				Azure: &garden.AzureCloud{},
			}

			cloudProvider, err := DetermineCloudProviderInShoot(cloud)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderAzure))
		})

		It("should return cloud provider GCP", func() {
			cloud := garden.Cloud{
				GCP: &garden.GCPCloud{},
			}

			cloudProvider, err := DetermineCloudProviderInShoot(cloud)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderGCP))
		})

		It("should return cloud provider OpenStack", func() {
			cloud := garden.Cloud{
				OpenStack: &garden.OpenStackCloud{},
			}

			cloudProvider, err := DetermineCloudProviderInShoot(cloud)

			Expect(err).NotTo(HaveOccurred())
			Expect(cloudProvider).To(Equal(garden.CloudProviderOpenStack))
		})

		It("should return an error because no cloud provider is set", func() {
			cloud := garden.Cloud{}

			_, err := DetermineCloudProviderInShoot(cloud)

			Expect(err).To(HaveOccurred())
		})

		It("should return an error because too many cloud providers are set", func() {
			cloud := garden.Cloud{
				AWS:   &garden.AWSCloud{},
				Azure: &garden.AzureCloud{},
			}

			_, err := DetermineCloudProviderInShoot(cloud)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DetermineLatestMachineImage for images with different names", func() {
		It("should return the Machine Images containing the latest image version", func() {
			images := []garden.MachineImage{
				{
					Name: "coreos",
					Versions: []garden.MachineImageVersion{
						{
							Version: "1.1",
						},
						{
							Version: "0.0.2",
						},
						{
							Version: "0.0.1",
						},
					},
				},

				{
					Name: "xy",
					Versions: []garden.MachineImageVersion{
						{
							Version: "2.1",
						},
						{
							Version: "1.0.0",
						},
						{
							Version: "1.0.1",
						},
					},
				},
			}

			latestImages, err := DetermineLatestMachineImageVersions(images)
			Expect(err).NotTo(HaveOccurred())

			Expect(latestImages).ToNot(BeNil())
			Expect(latestImages).ToNot(BeEmpty())
			Expect(latestImages).To(HaveLen(2))

			Expect(latestImages["xy"]).To(Equal(
				garden.MachineImageVersion{
					Version: "2.1",
				},
			))

			Expect(latestImages["coreos"]).To(Equal(
				garden.MachineImageVersion{
					Version: "1.1",
				},
			))
		})

		It("should return an error for invalid semVerVersion", func() {
			images := []garden.MachineImage{
				{
					Name: "coreos",
					Versions: []garden.MachineImageVersion{
						{
							Version: "1.1",
						},
						{
							Version: "0.0.2",
						},
						{
							Version: "0.0.1",
						},
					},
				},
				{
					Name: "xy",
					Versions: []garden.MachineImageVersion{
						{
							Version: "0.xx.0",
						},
					},
				},
			}

			_, err := DetermineLatestMachineImageVersions(images)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DetermineLatestKubernetesVersion", func() {
		It("should return the latest Kubernetes version", func() {
			offeredVersions := []garden.KubernetesVersion{
				{
					Version: "1.0.0",
				},
				{
					Version: "0.0.1",
				},
				{
					Version: "0.2.1",
				},
			}

			offeredVersion, err := DetermineLatestKubernetesVersion(offeredVersions)
			Expect(err).ToNot(HaveOccurred())
			Expect(offeredVersion.Version).To(Equal("1.0.0"))
		})
	})

	var (
		trueVar  = true
		falseVar = false
	)

	DescribeTable("#ShootWantsBasicAuthentication",
		func(kubeAPIServerConfig *garden.KubeAPIServerConfig, wantsBasicAuth bool) {
			actualWantsBasicAuth := ShootWantsBasicAuthentication(kubeAPIServerConfig)

			Expect(actualWantsBasicAuth).To(Equal(wantsBasicAuth))
		},
		Entry("no kubeapiserver configuration", nil, true),
		Entry("field not set", &garden.KubeAPIServerConfig{}, true),
		Entry("explicitly enabled", &garden.KubeAPIServerConfig{EnableBasicAuthentication: &trueVar}, true),
		Entry("explicitly disabled", &garden.KubeAPIServerConfig{EnableBasicAuthentication: &falseVar}, false),
	)

	DescribeTable("#TaintsHave",
		func(taints []garden.SeedTaint, key string, expectation bool) {
			Expect(TaintsHave(taints, key)).To(Equal(expectation))
		},
		Entry("taint exists", []garden.SeedTaint{{Key: "foo"}}, "foo", true),
		Entry("taint does not exist", []garden.SeedTaint{{Key: "foo"}}, "bar", false),
	)

	DescribeTable("#QuotaScope",
		func(apiVersion, kind, expectedScope string, expectedErr gomegatypes.GomegaMatcher) {
			scope, err := QuotaScope(corev1.ObjectReference{APIVersion: apiVersion, Kind: kind})
			Expect(scope).To(Equal(expectedScope))
			Expect(err).To(expectedErr)
		},

		Entry("project", "core.gardener.cloud/v1alpha1", "Project", "project", BeNil()),
		Entry("secret", "v1", "Secret", "secret", BeNil()),
		Entry("unknown", "v2", "Foo", "", HaveOccurred()),
	)
})
