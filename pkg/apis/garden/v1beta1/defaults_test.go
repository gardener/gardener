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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("#SetDefaults_Shoot", func() {
	var (
		shoot      *v1beta1.Shoot
		networking = &v1beta1.Networking{}
	)

	JustBeforeEach(func() {
		shoot.Spec.Networking = networking
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
			Context("kube controller - NodeCIDRMask", func() {
				Context("Non-nil kube controller NodeCIDRMask", func() {
					size := 23
					BeforeEach(func() {
						shoot.Spec.Kubernetes.KubeControllerManager = &v1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: &size}
					})

					It("NodeCIDRMask should be unchanged", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&size))
					})
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						BeforeEach(func() {
							var maxPod int32 = 260
							aws.Workers = []v1beta1.AWSWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
							}
						})
						It("should calculate an appropriate NodeCIDRMask", func() {
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
							expectedSize := 22
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})

					Context("multiple worker pools", func() {
						BeforeEach(func() {
							var maxPod int32 = 150
							var defaultMaxPod int32 = 110
							aws.Workers = []v1beta1.AWSWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should use the highest maxPod in any worker", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
					Context("kubernetes.Kubelet global default max pod", func() {
						BeforeEach(func() {
							var (
								maxPod        int32 = 150
								defaultMaxPod int32 = 110
							)
							shoot.Spec.Kubernetes.Kubelet = &v1beta1.KubeletConfig{
								MaxPods: &maxPod,
							}
							aws.Workers = []v1beta1.AWSWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should consider the global maxPod setting in spec.Kubernetes.Kubelet", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
				})

				Context("default max pod settings", func() {
					It("should calculate an appropriate NodeCIDRMask", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
						expectedSize := 24
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
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
				shoot.Spec.Networking = networking
			})

			Context("Shoot Networks", func() {
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

			Context("kube controller - NodeCIDRMask", func() {

				Context("Non-nil kube controller NodeCIDRMask", func() {
					size := 23
					BeforeEach(func() {
						shoot.Spec.Kubernetes.KubeControllerManager = &v1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: &size}
					})

					It("NodeCIDRMask should be unchanged", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&size))
					})
				})

				Context("Non-default max pod settings", func() {

					Context("one worker pool", func() {
						BeforeEach(func() {
							var maxPod int32 = 260
							azure.Workers = []v1beta1.AzureWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
							}
						})
						It("should calculate an appropriate NodeCIDRMask", func() {
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
							expectedSize := 22
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})

					Context("multiple worker pools", func() {
						BeforeEach(func() {
							var maxPod int32 = 150
							var defaultMaxPod int32 = 110
							azure.Workers = []v1beta1.AzureWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should use the highest maxPod in any worker", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
					Context("kubernetes.Kubelet global default max pod", func() {
						BeforeEach(func() {
							var (
								maxPod        int32 = 150
								defaultMaxPod int32 = 110
							)
							shoot.Spec.Kubernetes.Kubelet = &v1beta1.KubeletConfig{
								MaxPods: &maxPod,
							}
							azure.Workers = []v1beta1.AzureWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should consider the global maxPod setting in spec.Kubernetes.Kubelet", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
				})

				Context("default max pod settings", func() {
					It("should calculate an appropriate NodeCIDRMask", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
						expectedSize := 24
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
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
			Context("kube controller - NodeCIDRMask", func() {

				Context("Non-nil kube controller NodeCIDRMask", func() {
					size := 23
					BeforeEach(func() {
						shoot.Spec.Kubernetes.KubeControllerManager = &v1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: &size}
					})

					It("NodeCIDRMask should be unchanged", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&size))
					})
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						BeforeEach(func() {
							var maxPod int32 = 260
							gcp.Workers = []v1beta1.GCPWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
							}
						})
						It("should calculate an appropriate NodeCIDRMask", func() {
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
							expectedSize := 22
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})

					Context("multiple worker pools", func() {
						BeforeEach(func() {
							var maxPod int32 = 150
							var defaultMaxPod int32 = 110
							gcp.Workers = []v1beta1.GCPWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should use the highest maxPod in any worker", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
					Context("kubernetes.Kubelet global default max pod", func() {
						BeforeEach(func() {
							var (
								maxPod        int32 = 150
								defaultMaxPod int32 = 110
							)
							shoot.Spec.Kubernetes.Kubelet = &v1beta1.KubeletConfig{
								MaxPods: &maxPod,
							}
							gcp.Workers = []v1beta1.GCPWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should consider the global maxPod setting in spec.Kubernetes.Kubelet", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
				})

				Context("default max pod settings", func() {
					It("should calculate an appropriate NodeCIDRMask", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
						expectedSize := 24
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
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

			Context("kube controller - NodeCIDRMask", func() {

				Context("Non-nil kube controller NodeCIDRMask", func() {
					size := 23
					BeforeEach(func() {
						shoot.Spec.Kubernetes.KubeControllerManager = &v1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: &size}
					})

					It("NodeCIDRMask should be unchanged", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&size))
					})
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						BeforeEach(func() {
							var maxPod int32 = 260
							alicloud.Workers = []v1beta1.AlicloudWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
							}
						})
						It("should calculate an appropriate NodeCIDRMask", func() {
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
							expectedSize := 22
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})

					Context("multiple worker pools", func() {
						BeforeEach(func() {
							var maxPod int32 = 150
							var defaultMaxPod int32 = 110
							alicloud.Workers = []v1beta1.AlicloudWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should use the highest maxPod in any worker", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
					Context("kubernetes.Kubelet global default max pod", func() {
						BeforeEach(func() {
							var (
								maxPod        int32 = 150
								defaultMaxPod int32 = 110
							)
							shoot.Spec.Kubernetes.Kubelet = &v1beta1.KubeletConfig{
								MaxPods: &maxPod,
							}
							alicloud.Workers = []v1beta1.AlicloudWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should consider the global maxPod setting in spec.Kubernetes.Kubelet", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
				})

				Context("default max pod settings", func() {
					It("should calculate an appropriate NodeCIDRMask", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
						expectedSize := 24
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
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
				shoot.Spec.Networking = networking
			})

			Context("Shoot Networks", func() {
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
			Context("kube controller - NodeCIDRMask", func() {

				Context("Non-nil kube controller NodeCIDRMask", func() {
					size := 23
					BeforeEach(func() {
						shoot.Spec.Kubernetes.KubeControllerManager = &v1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: &size}
					})

					It("NodeCIDRMask should be unchanged", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&size))
					})
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						BeforeEach(func() {
							var maxPod int32 = 260
							openstack.Workers = []v1beta1.OpenStackWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
							}
						})
						It("should calculate an appropriate NodeCIDRMask", func() {
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
							expectedSize := 22
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})

					Context("multiple worker pools", func() {
						BeforeEach(func() {
							var maxPod int32 = 150
							var defaultMaxPod int32 = 110
							openstack.Workers = []v1beta1.OpenStackWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should use the highest maxPod in any worker", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
					Context("kubernetes.Kubelet global default max pod", func() {
						BeforeEach(func() {
							var (
								maxPod        int32 = 150
								defaultMaxPod int32 = 110
							)
							shoot.Spec.Kubernetes.Kubelet = &v1beta1.KubeletConfig{
								MaxPods: &maxPod,
							}
							openstack.Workers = []v1beta1.OpenStackWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should consider the global maxPod setting in spec.Kubernetes.Kubelet", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
				})

				Context("default max pod settings", func() {
					It("should calculate an appropriate NodeCIDRMask", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
						expectedSize := 24
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
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

			Context("kube controller - NodeCIDRMask", func() {

				Context("Non-nil kube controller NodeCIDRMask", func() {
					size := 23
					BeforeEach(func() {
						shoot.Spec.Kubernetes.KubeControllerManager = &v1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: &size}
					})

					It("NodeCIDRMask should be unchanged", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&size))
					})
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						BeforeEach(func() {
							var maxPod int32 = 260
							shoot.Spec.Cloud.Packet.Workers = []v1beta1.PacketWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
							}
						})
						It("should calculate an appropriate NodeCIDRMask", func() {
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
							expectedSize := 22
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})

					Context("multiple worker pools", func() {
						BeforeEach(func() {
							var maxPod int32 = 150
							var defaultMaxPod int32 = 110
							shoot.Spec.Cloud.Packet.Workers = []v1beta1.PacketWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &maxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should use the highest maxPod in any worker", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
					Context("kubernetes.Kubelet global default max pod", func() {
						BeforeEach(func() {
							var (
								maxPod        int32 = 150
								defaultMaxPod int32 = 110
							)
							shoot.Spec.Kubernetes.Kubelet = &v1beta1.KubeletConfig{
								MaxPods: &maxPod,
							}
							shoot.Spec.Cloud.Packet.Workers = []v1beta1.PacketWorker{
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
								{
									Worker: v1beta1.Worker{
										Kubelet: &v1beta1.KubeletConfig{
											MaxPods: &defaultMaxPod,
										},
									},
								},
							}
						})
						It("should consider the global maxPod setting in spec.Kubernetes.Kubelet", func() {
							expectedSize := 23
							Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
						})
					})
				})

				Context("default max pod settings", func() {
					It("should calculate an appropriate NodeCIDRMask", func() {
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).ToNot(BeNil())
						expectedSize := 24
						Expect(shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(&expectedSize))
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

			Context("without provided maintenance", func() {
				It("should automatically update the Kubernetes version", func() {
					Expect(shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
				})

				It("should have a valid maintenance start time", func() {
					Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.Begin)).ShouldNot(PointTo(BeNil()))
				})

				It("should have a valid maintenance end time", func() {
					Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.End)).ShouldNot(PointTo(BeNil()))
				})
			})

			Context("with provided maintenance", func() {
				var maintenance *v1beta1.Maintenance

				BeforeEach(func() {
					maintenance = &v1beta1.Maintenance{}
					shoot.Spec.Maintenance = maintenance
				})

				It("should automatically update the Kubernetes version", func() {
					Expect(shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
				})

				It("should have a valid maintenance start time", func() {
					Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.Begin)).ShouldNot(PointTo(BeNil()))
				})

				It("should have a valid maintenance end time", func() {
					Expect(utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.End)).ShouldNot(PointTo(BeNil()))
				})
			})

		})
	})
})
