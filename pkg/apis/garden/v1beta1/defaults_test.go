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

package v1beta1_test

import (
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("#SetDefaults_Shoot", func() {
	var (
		shoot *v1beta1.Shoot
	)

	JustBeforeEach(func() {
		v1beta1.SetDefaults_Shoot(shoot)
	})

	BeforeEach(func() {
		shoot = &v1beta1.Shoot{}
	})

	Context("cloud", func() {

		Context("aws", func() {
			var (
				aws *v1beta1.AWSCloud
			)
			BeforeEach(func() {
				aws = &v1beta1.AWSCloud{}
				shoot.Spec.Cloud.AWS = aws
			})

			Context("Shoot Networks", func() {

				It("should set default Pod CIDR", func() {
					Expect(shoot.Spec.Cloud.AWS.Networks.Pods).To(PointTo(Equal(v1alpha1.CIDR("100.96.0.0/11"))))
				})

				It("should set default Services CIDR", func() {
					Expect(shoot.Spec.Cloud.AWS.Networks.Services).To(PointTo(Equal(v1alpha1.CIDR("100.64.0.0/13"))))
				})

				Context("nodes", func() {
					Context("provided VPC CIDR", func() {
						const vpcCIDR = v1alpha1.CIDR("1.1.0.0/24")
						BeforeEach(func() {
							cidr := vpcCIDR
							aws.Networks.VPC.CIDR = &cidr
						})

						It("should set the nodes to the VPC CIDR", func() {
							Expect(shoot.Spec.Cloud.AWS.Networks.Nodes).To(PointTo(Equal(vpcCIDR)))
						})
					})

					Context("not provide VPC CIDR", func() {
						Context("without any workers", func() {
							It("should be nil", func() {
								Expect(shoot.Spec.Cloud.AWS.Networks.Nodes).To(BeNil())
							})
						})

						Context("with one worker", func() {
							const workerCIDR = v1alpha1.CIDR("1.1.0.0/24")

							BeforeEach(func() {
								aws.Networks.Workers = []v1alpha1.CIDR{workerCIDR}
							})

							It("should be the same value as aws.networks.workers[0]", func() {
								Expect(shoot.Spec.Cloud.AWS.Networks.Nodes).To(PointTo(Equal(workerCIDR)))
							})
						})

						Context("with more than one workers", func() {
							BeforeEach(func() {
								aws.Networks.Workers = []v1alpha1.CIDR{"1.1.0.0/24", "1.1.2.0/24"}
							})

							It("should be nil", func() {
								Expect(shoot.Spec.Cloud.AWS.Networks.Nodes).To(BeNil())
							})
						})
					})
				})
			})
		})

		Context("azure", func() {
			var (
				azure *v1beta1.AzureCloud
			)
			BeforeEach(func() {
				azure = &v1beta1.AzureCloud{}
				shoot.Spec.Cloud.Azure = azure
			})

			Context("Shoot Networks", func() {

				It("should set default Pod CIDR", func() {
					Expect(shoot.Spec.Cloud.Azure.Networks.Pods).To(PointTo(Equal(v1alpha1.CIDR("100.96.0.0/11"))))
				})

				It("should set default Services CIDR", func() {
					Expect(shoot.Spec.Cloud.Azure.Networks.Services).To(PointTo(Equal(v1alpha1.CIDR("100.64.0.0/13"))))
				})

				Context("with one worker", func() {
					const workerCIDR = v1alpha1.CIDR("1.1.0.0/24")

					BeforeEach(func() {
						azure.Networks.Workers = workerCIDR
					})

					It("should be the same value as azure.networks.workers[0]", func() {
						Expect(shoot.Spec.Cloud.Azure.Networks.Nodes).To(PointTo(Equal(workerCIDR)))
					})
				})
			})
		})

		Context("gcp", func() {
			var (
				gcp *v1beta1.GCPCloud
			)
			BeforeEach(func() {
				gcp = &v1beta1.GCPCloud{}
				shoot.Spec.Cloud.GCP = gcp
			})

			Context("Shoot Networks", func() {

				It("should set default Pod CIDR", func() {
					Expect(shoot.Spec.Cloud.GCP.Networks.Pods).To(PointTo(Equal(v1alpha1.CIDR("100.96.0.0/11"))))
				})

				It("should set default Services CIDR", func() {
					Expect(shoot.Spec.Cloud.GCP.Networks.Services).To(PointTo(Equal(v1alpha1.CIDR("100.64.0.0/13"))))
				})

				Context("nodes", func() {

					Context("without any workers", func() {
						It("should be nil", func() {
							Expect(shoot.Spec.Cloud.GCP.Networks.Nodes).To(BeNil())
						})
					})

					Context("with one worker", func() {
						const workerCIDR = v1alpha1.CIDR("1.1.0.0/24")

						BeforeEach(func() {
							gcp.Networks.Workers = []v1alpha1.CIDR{workerCIDR}
						})

						It("should be the same value as gcp.networks.workers[0]", func() {
							Expect(shoot.Spec.Cloud.GCP.Networks.Nodes).To(PointTo(Equal(workerCIDR)))
						})
					})

					Context("with more than one workers", func() {
						BeforeEach(func() {
							gcp.Networks.Workers = []v1alpha1.CIDR{"1.1.0.0/24", "1.1.2.0/24"}
						})

						It("should be nil", func() {
							Expect(shoot.Spec.Cloud.GCP.Networks.Nodes).To(BeNil())
						})
					})
				})
			})
		})

		Context("alicloud", func() {
			var (
				alicloud *v1beta1.Alicloud
			)
			BeforeEach(func() {
				alicloud = &v1beta1.Alicloud{}
				shoot.Spec.Cloud.Alicloud = alicloud
			})

			Context("Shoot Networks", func() {

				It("should set default Pod CIDR", func() {
					Expect(shoot.Spec.Cloud.Alicloud.Networks.Pods).To(PointTo(Equal(v1alpha1.CIDR("100.64.0.0/11"))))
				})

				It("should set default Services CIDR", func() {
					Expect(shoot.Spec.Cloud.Alicloud.Networks.Services).To(PointTo(Equal(v1alpha1.CIDR("100.104.0.0/13"))))
				})

				Context("nodes", func() {
					Context("provided VPC CIDR", func() {
						const vpcCIDR = v1alpha1.CIDR("1.1.0.0/24")
						BeforeEach(func() {
							cidr := vpcCIDR
							alicloud.Networks.VPC.CIDR = &cidr
						})

						It("should set the nodes to the VPC CIDR", func() {
							Expect(shoot.Spec.Cloud.Alicloud.Networks.Nodes).To(PointTo(Equal(vpcCIDR)))
						})
					})

					Context("not provide VPC CIDR", func() {
						Context("without any workers", func() {
							It("should be nil", func() {
								Expect(shoot.Spec.Cloud.Alicloud.Networks.Nodes).To(BeNil())
							})
						})

						Context("with one worker", func() {
							const workerCIDR = v1alpha1.CIDR("1.1.0.0/24")

							BeforeEach(func() {
								alicloud.Networks.Workers = []v1alpha1.CIDR{workerCIDR}
							})

							It("should be the same value as alicloud.networks.workers[0]", func() {
								Expect(shoot.Spec.Cloud.Alicloud.Networks.Nodes).To(PointTo(Equal(workerCIDR)))
							})
						})

						Context("with more than one workers", func() {
							BeforeEach(func() {
								alicloud.Networks.Workers = []v1alpha1.CIDR{"1.1.0.0/24", "1.1.2.0/24"}
							})

							It("should be nil", func() {
								Expect(shoot.Spec.Cloud.Alicloud.Networks.Nodes).To(BeNil())
							})
						})
					})
				})
			})
		})

		Context("openstack", func() {
			var (
				openstack *v1beta1.OpenStackCloud
			)
			BeforeEach(func() {
				openstack = &v1beta1.OpenStackCloud{}
				shoot.Spec.Cloud.OpenStack = openstack
			})

			Context("Shoot Networks", func() {

				It("should set default Pod CIDR", func() {
					Expect(shoot.Spec.Cloud.OpenStack.Networks.Pods).To(PointTo(Equal(v1alpha1.CIDR("100.96.0.0/11"))))
				})

				It("should set default Services CIDR", func() {
					Expect(shoot.Spec.Cloud.OpenStack.Networks.Services).To(PointTo(Equal(v1alpha1.CIDR("100.64.0.0/13"))))
				})

				Context("nodes", func() {

					Context("without any workers", func() {
						It("should be nil", func() {
							Expect(shoot.Spec.Cloud.OpenStack.Networks.Nodes).To(BeNil())
						})
					})

					Context("with one worker", func() {
						const workerCIDR = v1alpha1.CIDR("1.1.0.0/24")

						BeforeEach(func() {
							openstack.Networks.Workers = []v1alpha1.CIDR{workerCIDR}
						})

						It("should be the same value as openstack.networks.workers[0]", func() {
							Expect(shoot.Spec.Cloud.OpenStack.Networks.Nodes).To(PointTo(Equal(workerCIDR)))
						})
					})

					Context("with more than one workers", func() {
						BeforeEach(func() {
							openstack.Networks.Workers = []v1alpha1.CIDR{"1.1.0.0/24", "1.1.2.0/24"}
						})

						It("should be nil", func() {
							Expect(shoot.Spec.Cloud.OpenStack.Networks.Nodes).To(BeNil())
						})
					})
				})
			})
		})

		Context("packet", func() {
			var (
				packet *v1beta1.PacketCloud
			)
			BeforeEach(func() {
				packet = &v1beta1.PacketCloud{}
				shoot.Spec.Cloud.Packet = packet
			})

			Context("Shoot Networks", func() {

				It("should set default Pod CIDR", func() {
					Expect(shoot.Spec.Cloud.Packet.Networks.Pods).To(PointTo(Equal(v1alpha1.CIDR("100.96.0.0/11"))))
				})

				It("should set default Services CIDR", func() {
					Expect(shoot.Spec.Cloud.Packet.Networks.Services).To(PointTo(Equal(v1alpha1.CIDR("100.64.0.0/13"))))
				})
			})
		})
	})

	Context("kubernetes", func() {
		It("should enable privileged containers by default", func() {
			Expect(shoot.Spec.Kubernetes.AllowPrivilegedContainers).To(PointTo(BeTrue()))
		})

		Context("kubeproxy", func() {
			// TODO: Fix this in next API version of the Shoot spec.
			It("should use not set kube-proxy to any value", func() {
				Expect(shoot.Spec.Kubernetes.KubeProxy).To(BeNil())
			})
			Context("when kubeProxy is not nil", func() {
				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeProxy = &v1beta1.KubeProxyConfig{}
				})
				It("should use iptables as default mode", func() {
					// Don't change this value to guarantee backwards compatibility.
					defaultMode := v1beta1.ProxyMode("IPTables")
					Expect(shoot.Spec.Kubernetes.KubeProxy.Mode).To(PointTo(Equal(defaultMode)))
				})
			})

		})
	})

	Context("maintenance", func() {

		Context("without provided maitanence", func() {
			It("should automatically update the Kubernetes version", func() {
				Expect(shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
			})

			It("should have a valid mataince start time", func() {
				Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.Begin)).ShouldNot(PointTo(BeNil()))
			})

			It("should have a valid mataince end time", func() {
				Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.End)).ShouldNot(PointTo(BeNil()))
			})
		})

		Context("with provided maitanence", func() {
			var maintenance *v1beta1.Maintenance

			BeforeEach(func() {
				maintenance = &v1beta1.Maintenance{}
				shoot.Spec.Maintenance = maintenance
			})

			It("should automatically update the Kubernetes version", func() {
				Expect(shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
			})

			It("should have a valid mataince start time", func() {
				Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.Begin)).ShouldNot(PointTo(BeNil()))
			})

			It("should have a valid mataince end time", func() {
				Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.End)).ShouldNot(PointTo(BeNil()))
			})
		})

	})
})
