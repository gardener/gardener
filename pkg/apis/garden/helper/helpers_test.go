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
	. "github.com/onsi/gomega"
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
})
