// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"context"
	"testing"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/garden"
	strategy "github.com/gardener/gardener/pkg/registry/garden/shoot"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoot Suite")
}

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := strategy.ToSelectableFields(newShoot("foo"))

		Expect(result).To(HaveLen(3))
		Expect(result.Has(garden.ShootSeedName)).To(BeTrue())
		Expect(result.Get(garden.ShootSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not Shoot", func() {
		_, _, err := strategy.GetAttrs(&garden.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := strategy.GetAttrs(newShoot("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(garden.ShootSeedName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedTriggerFunc", func() {
	It("should return correct matching values", func() {
		expected := []storage.MatchValue{{IndexName: garden.ShootSeedName, Value: "foo"}}

		mv := strategy.SeedTriggerFunc(newShoot("foo"))

		Expect(mv).To(Equal(expected))

	})
})

var _ = Describe("MatchShoot", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(garden.ShootSeedName, "foo")

		result := strategy.MatchShoot(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(garden.ShootSeedName))

	})
})

var _ = Describe("Strategy", func() {

	Context("PrepareForUpdate", func() {
		Context("invalid GCP network CIRDs", func() {
			It("should remove more than one GCP networks", func() {
				shoot := newShoot("foo")

				shoot.Spec.Cloud.GCP = &garden.GCPCloud{
					Networks: garden.GCPNetworks{
						Workers: []core.CIDR{"1.1.1.1/32", "1.1.1.2/32"},
					},
				}
				oldShoot := newShoot("foo")
				oldShoot.Spec.Cloud.GCP = shoot.Spec.Cloud.GCP.DeepCopy()

				strategy.Strategy.PrepareForUpdate(context.TODO(), shoot, oldShoot)

				Expect(shoot.Spec.Cloud.GCP.Networks.Workers).To(ConsistOf(core.CIDR("1.1.1.1/32")))
				Expect(oldShoot.Spec.Cloud.GCP.Networks.Workers).To(ConsistOf(core.CIDR("1.1.1.1/32")))
			})
		})
	})

	Context("Canonicalize", func() {
		It("should convert all network entries to connonical reprentation", func() {

			given := garden.Shoot{
				Spec: garden.ShootSpec{
					Networking: &garden.Networking{
						K8SNetworks: core.K8SNetworks{
							Nodes:    cidrPtr("10.0.0.2/24"),
							Pods:     cidrPtr("10.0.1.3/24"),
							Services: cidrPtr("10.0.2.4/24"),
						},
					},
					Cloud: garden.Cloud{
						AWS: &garden.AWSCloud{
							Networks: garden.AWSNetworks{
								K8SNetworks: core.K8SNetworks{
									Nodes:    cidrPtr("10.0.3.2/24"),
									Pods:     cidrPtr("10.0.4.3/24"),
									Services: cidrPtr("10.0.5.4/24"),
								},
								VPC: garden.AWSVPC{
									CIDR: cidrPtr("10.0.6.5/24"),
								},
								Internal: makeCIDRs("10.0.7.6/24", "10.0.8.7/24"),
								Public:   makeCIDRs("10.0.8.7/24", "10.0.9.8/24"),
								Workers:  makeCIDRs("10.0.9.8/24", "10.0.10.9/24"),
							},
						},
						Azure: &garden.AzureCloud{
							Networks: garden.AzureNetworks{
								K8SNetworks: core.K8SNetworks{
									Nodes:    cidrPtr("10.0.11.10/24"),
									Pods:     cidrPtr("10.0.12.11/24"),
									Services: cidrPtr("10.0.13.12/24"),
								},
								VNet: garden.AzureVNet{
									CIDR: cidrPtr("10.0.14.13/24"),
								},
								Workers: core.CIDR("10.0.15.14/24"),
							},
						},

						GCP: &garden.GCPCloud{
							Networks: garden.GCPNetworks{
								K8SNetworks: core.K8SNetworks{
									Nodes:    cidrPtr("10.0.16.15/24"),
									Pods:     cidrPtr("10.0.17.16/24"),
									Services: cidrPtr("10.0.18.17/24"),
								},
								Internal: cidrPtr("10.0.19.18/24"),
								Workers:  makeCIDRs("10.0.20.19/24", "10.0.21.20/24"),
							},
						},
						OpenStack: &garden.OpenStackCloud{
							Networks: garden.OpenStackNetworks{
								K8SNetworks: core.K8SNetworks{
									Nodes:    cidrPtr("10.0.22.21/24"),
									Pods:     cidrPtr("10.0.23.22/24"),
									Services: cidrPtr("10.0.24.23/24"),
								},
								Workers: makeCIDRs("10.0.25.24/24", "10.0.26.25/24"),
							},
						},

						Alicloud: &garden.Alicloud{
							Networks: garden.AlicloudNetworks{
								K8SNetworks: core.K8SNetworks{
									Nodes:    cidrPtr("10.0.27.26/24"),
									Pods:     cidrPtr("10.0.28.27/24"),
									Services: cidrPtr("10.0.29.28/24"),
								},
								VPC: garden.AlicloudVPC{
									CIDR: cidrPtr("10.0.30.29/24"),
								},
								Workers: makeCIDRs("10.0.31.30/24", "10.0.32.31/24"),
							},
						},
						Packet: &garden.PacketCloud{
							Networks: garden.PacketNetworks{
								K8SNetworks: core.K8SNetworks{
									Nodes:    cidrPtr("10.0.33.32/24"),
									Pods:     cidrPtr("10.0.34.33/24"),
									Services: cidrPtr("10.0.35.34/24"),
								},
							},
						},
					},
				},
			}
			expected := given.DeepCopy()

			strategy.Strategy.Canonicalize(&given)

			// spec.networks
			expected.Spec.Networking.Nodes = cidrPtr("10.0.0.0/24")
			expected.Spec.Networking.Pods = cidrPtr("10.0.1.0/24")
			expected.Spec.Networking.Services = cidrPtr("10.0.2.0/24")

			// spec.cloud.aws
			expected.Spec.Cloud.AWS.Networks.Nodes = cidrPtr("10.0.3.0/24")
			expected.Spec.Cloud.AWS.Networks.Pods = cidrPtr("10.0.4.0/24")
			expected.Spec.Cloud.AWS.Networks.Services = cidrPtr("10.0.5.0/24")
			expected.Spec.Cloud.AWS.Networks.VPC.CIDR = cidrPtr("10.0.6.0/24")
			expected.Spec.Cloud.AWS.Networks.Internal = makeCIDRs("10.0.7.0/24", "10.0.8.0/24")
			expected.Spec.Cloud.AWS.Networks.Public = makeCIDRs("10.0.8.0/24", "10.0.9.0/24")
			expected.Spec.Cloud.AWS.Networks.Workers = makeCIDRs("10.0.9.0/24", "10.0.10.0/24")

			// spec.cloud.azure
			expected.Spec.Cloud.Azure.Networks.Nodes = cidrPtr("10.0.11.0/24")
			expected.Spec.Cloud.Azure.Networks.Pods = cidrPtr("10.0.12.0/24")
			expected.Spec.Cloud.Azure.Networks.Services = cidrPtr("10.0.13.0/24")
			expected.Spec.Cloud.Azure.Networks.VNet.CIDR = cidrPtr("10.0.14.0/24")
			expected.Spec.Cloud.Azure.Networks.Workers = core.CIDR("10.0.15.0/24")

			// spec.cloud.gcp
			expected.Spec.Cloud.GCP.Networks.Nodes = cidrPtr("10.0.16.0/24")
			expected.Spec.Cloud.GCP.Networks.Pods = cidrPtr("10.0.17.0/24")
			expected.Spec.Cloud.GCP.Networks.Services = cidrPtr("10.0.18.0/24")
			expected.Spec.Cloud.GCP.Networks.Internal = cidrPtr("10.0.19.0/24")
			expected.Spec.Cloud.GCP.Networks.Workers = makeCIDRs("10.0.20.0/24", "10.0.21.0/24")

			// spec.cloud.openstack
			expected.Spec.Cloud.OpenStack.Networks.Nodes = cidrPtr("10.0.22.0/24")
			expected.Spec.Cloud.OpenStack.Networks.Pods = cidrPtr("10.0.23.0/24")
			expected.Spec.Cloud.OpenStack.Networks.Services = cidrPtr("10.0.24.0/24")
			expected.Spec.Cloud.OpenStack.Networks.Workers = makeCIDRs("10.0.25.0/24", "10.0.26.0/24")

			// spec.cloud.alicloud
			expected.Spec.Cloud.Alicloud.Networks.Nodes = cidrPtr("10.0.27.0/24")
			expected.Spec.Cloud.Alicloud.Networks.Pods = cidrPtr("10.0.28.0/24")
			expected.Spec.Cloud.Alicloud.Networks.Services = cidrPtr("10.0.29.0/24")
			expected.Spec.Cloud.Alicloud.Networks.VPC.CIDR = cidrPtr("10.0.30.0/24")
			expected.Spec.Cloud.Alicloud.Networks.Workers = makeCIDRs("10.0.31.0/24", "10.0.32.0/24")

			// spec.cloud.packet
			expected.Spec.Cloud.Packet.Networks.Nodes = cidrPtr("10.0.33.0/24")
			expected.Spec.Cloud.Packet.Networks.Pods = cidrPtr("10.0.34.0/24")
			expected.Spec.Cloud.Packet.Networks.Services = cidrPtr("10.0.35.0/24")

			Expect(*expected).To(BeEquivalentTo(given))

		})
	})

})

func newShoot(seedName string) *garden.Shoot {
	return &garden.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: garden.ShootSpec{
			Cloud: garden.Cloud{
				Seed: &seedName,
			},
		},
	}
}

func cidrPtr(cidr string) *core.CIDR {
	c := core.CIDR(cidr)
	return &c
}

func makeCIDRs(cidrs ...string) []core.CIDR {
	converted := make([]core.CIDR, 0, len(cidrs))
	for _, cidr := range cidrs {
		converted = append(converted, core.CIDR(cidr))
	}
	return converted
}
