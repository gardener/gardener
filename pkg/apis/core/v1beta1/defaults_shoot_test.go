// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("Shoot defaulting", func() {
	var obj *Shoot

	BeforeEach(func() {
		obj = &Shoot{
			Spec: ShootSpec{
				Kubernetes: Kubernetes{
					Version: "1.26.1",
				},
				Provider: Provider{
					Workers: []Worker{{}},
				},
			},
		}
	})

	Describe("Kubernetes defaulting", func() {
		It("should set the kubeScheduler field for Shoot with workers", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeScheduler).NotTo(BeNil())
		})

		It("should not set the kubeScheduler field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeScheduler).To(BeNil())
		})

		It("should set the kubeProxy field for Shoot with workers", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeProxy).NotTo(BeNil())
		})

		It("should not set the kubeProxy field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeProxy).To(BeNil())
		})

		It("should set the kubelet field for Shoot with workers", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet).NotTo(BeNil())
		})

		It("should not set the kubelet field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet).To(BeNil())
		})

		It("should default the failSwapOn field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeTrue()))
		})

		It("should not default the failSwapOn field", func() {
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.FailSwapOn = pointer.Bool(false)

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeFalse()))
		})

		It("should default the swap behaviour", func() {
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.FailSwapOn = pointer.Bool(false)
			obj.Spec.Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap).To(Not(BeNil()))
			Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(PointTo(Equal(LimitedSwap)))
		})

		It("should not default the swap behaviour because failSwapOn=true", func() {
			trueVar := true
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.FailSwapOn = &trueVar
			obj.Spec.Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap).To(BeNil())
		})

		It("should not default the swap behaviour because kubelet feature gate NodeSwap is not set", func() {
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.FailSwapOn = pointer.Bool(false)
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap).To(BeNil())
		})

		It("should default the imageGCThreshold fields", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent).To(PointTo(Equal(int32(50))))
			Expect(obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent).To(PointTo(Equal(int32(40))))
		})

		It("should not default the imageGCThreshold fields", func() {
			var (
				high int32 = 12
				low  int32 = 34
			)

			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent = &high
			obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent = &low

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent).To(PointTo(Equal(high)))
			Expect(obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent).To(PointTo(Equal(low)))
		})

		It("should default the serializeImagePulls field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.SerializeImagePulls).To(PointTo(BeTrue()))
		})

		It("should not default the serializeImagePulls field if it is already set", func() {
			falseVar := false
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.SerializeImagePulls = &falseVar

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.SerializeImagePulls).To(PointTo(BeFalse()))
		})
	})

	Describe("Worker Swap", func() {
		It("should default the swap behaviour for a worker pool", func() {
			falseVar := false
			obj.Spec.Provider.Workers = []Worker{
				{
					Kubernetes: &WorkerKubernetes{
						Kubelet: &KubeletConfig{},
					},
				},
			}
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FailSwapOn = &falseVar
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap).To(Not(BeNil()))
			Expect(obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(PointTo(Equal(LimitedSwap)))
		})

		It("should not default the swap behaviour for a worker pool because failSwapOn=true (defaulted to true)", func() {
			obj.Spec.Provider.Workers = []Worker{
				{
					Kubernetes: &WorkerKubernetes{
						Kubelet: &KubeletConfig{},
					},
				},
			}
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap).To(BeNil())
		})

		It("should not default the swap behaviour for a worker pool because kubelet feature gate NodeSwap is not set", func() {
			falseVar := false
			obj.Spec.Provider.Workers = []Worker{
				{
					Kubernetes: &WorkerKubernetes{
						Kubelet: &KubeletConfig{},
					},
				},
			}
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FailSwapOn = &falseVar
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap).To(BeNil())
		})
	})

	Describe("Purpose defaulting", func() {
		It("should default purpose field", func() {
			obj.Spec.Purpose = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Purpose).To(PointTo(Equal(ShootPurposeEvaluation)))
		})

		It("should not default purpose field if it is already set", func() {
			p := ShootPurposeDevelopment
			obj.Spec.Purpose = &p

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Purpose).To(PointTo(Equal(ShootPurposeDevelopment)))
		})
	})

	Describe("Tolerations defaulting", func() {
		It("should not add the 'protected' toleration if the namespace is not 'garden'", func() {
			obj.Namespace = "foo"
			obj.Spec.Tolerations = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Tolerations).To(BeNil())
		})

		It("should add the 'protected' toleration if the namespace is 'garden'", func() {
			obj.Namespace = "garden"
			obj.Spec.Tolerations = []Toleration{{Key: "foo"}}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Tolerations).To(ConsistOf(
				Equal(Toleration{Key: "foo"}),
				Equal(Toleration{Key: SeedTaintProtected}),
			))
		})
	})

	Describe("SchedulerName defaulting", func() {
		It("should default schedulerName", func() {
			obj.Spec.SchedulerName = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SchedulerName).To(PointTo(Equal("default-scheduler")))
		})

		It("should not default schedulerName field if it is already set", func() {
			obj.Spec.SchedulerName = pointer.String("foo-scheduler")

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SchedulerName).To(PointTo(Equal("foo-scheduler")))
		})
	})

	Describe("KubeReserved defaulting", func() {
		var (
			defaultKubeReservedMemory = resource.MustParse("1Gi")
			defaultKubeReservedCPU    = resource.MustParse("80m")
			defaultKubeReservedPID    = resource.MustParse("20k")
			kubeReservedMemory        = resource.MustParse("2Gi")
			kubeReservedCPU           = resource.MustParse("20m")
			kubeReservedPID           = resource.MustParse("10k")
		)

		It("should default all fields", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.KubeReserved).To(PointTo(Equal(KubeletConfigReserved{
				CPU:    &defaultKubeReservedCPU,
				Memory: &defaultKubeReservedMemory,
				PID:    &defaultKubeReservedPID,
			})))
		})

		It("should not overwrite manually set kubeReserved", func() {
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{
				KubeReserved: &KubeletConfigReserved{
					CPU:    &kubeReservedCPU,
					Memory: &kubeReservedMemory,
					PID:    &kubeReservedPID,
				},
			}
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.KubeReserved).To(PointTo(Equal(KubeletConfigReserved{
				CPU:    &kubeReservedCPU,
				Memory: &kubeReservedMemory,
				PID:    &kubeReservedPID,
			})))
		})
	})

	Describe("KubeControllerManager settings defaulting", func() {
		Describe("NodeCIDRMaskSize", func() {
			It("should not default nodeCIDRMaskSize field for workerless Shoot", func() {
				obj.Spec.Provider.Workers = nil

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(BeNil())
			})

			Context("IPv4", func() {
				It("should make nodeCIDRMaskSize big enough for 2*maxPods", func() {
					obj.Spec.Provider.Workers = []Worker{{}}
					obj.Spec.Kubernetes.Kubelet = &KubeletConfig{
						MaxPods: pointer.Int32(250),
					}

					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(pointer.Int32(23)))
				})

				It("should make nodeCIDRMaskSize big enough for 2*maxPods (consider worker pool settings)", func() {
					obj.Spec.Kubernetes.Kubelet = &KubeletConfig{
						MaxPods: pointer.Int32(64),
					}
					obj.Spec.Provider.Workers = []Worker{{
						Name: "1",
						Kubernetes: &WorkerKubernetes{
							Kubelet: &KubeletConfig{
								MaxPods: pointer.Int32(100),
							},
						},
					}, {
						Name: "2",
						Kubernetes: &WorkerKubernetes{
							Kubelet: &KubeletConfig{
								MaxPods: pointer.Int32(260),
							},
						},
					}}

					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(pointer.Int32(22)))
				})
			})

			Context("IPv6", func() {
				BeforeEach(func() {
					obj.Spec.Provider.Workers = []Worker{{}}
					obj.Spec.Networking = &Networking{}
					obj.Spec.Networking.IPFamilies = []IPFamily{IPFamilyIPv6}
				})

				It("should default nodeCIDRMaskSize to 64", func() {
					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(pointer.Int32(64)))
				})
			})
		})
	})

	Describe("KubeScheduler defaulting", func() {
		It("should not default kubeScheduler field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeScheduler).To(BeNil())
		})

		It("should default the kubeScheduler.profile field", func() {
			obj.Spec.Kubernetes.KubeScheduler = &KubeSchedulerConfig{}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeScheduler.Profile).To(PointTo(Equal(SchedulingProfileBalanced)))
		})

		It("should not default the kubeScheduler.profile field if it is already set", func() {
			profile := SchedulingProfileBinPacking
			obj.Spec.Kubernetes.KubeScheduler = &KubeSchedulerConfig{
				Profile: &profile,
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeScheduler.Profile).To(PointTo(Equal(SchedulingProfileBinPacking)))
		})
	})

	Describe("Networking", func() {
		BeforeEach(func() {
			obj.Spec.Networking = nil
		})

		It("should set the networking field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Networking).NotTo(BeNil())
		})

		It("should default ipFamilies setting to IPv4 single-stack", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Networking).NotTo(BeNil())
			Expect(obj.Spec.Networking.IPFamilies).To(ConsistOf(IPFamilyIPv4))
		})
	})

	Describe("Addons defaulting", func() {
		It("should set the addons field for shoot with workers", func() {
			obj.Spec.Addons = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons).NotTo(BeNil())
		})

		It("should set the kubernetesDashboard field for shoot with workers", func() {
			obj.Spec.Addons = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons).NotTo(BeNil())
			Expect(obj.Spec.Addons.KubernetesDashboard).NotTo(BeNil())
			Expect(obj.Spec.Addons.KubernetesDashboard.AuthenticationMode).To(PointTo(Equal(KubernetesDashboardAuthModeToken)))
		})

		It("should set the nginxIngress field for shoot with workers", func() {
			obj.Spec.Addons = &Addons{}
			obj.Spec.Addons.NginxIngress = &NginxIngress{}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons.NginxIngress).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).To(PointTo(Equal(corev1.ServiceExternalTrafficPolicyTypeCluster)))
		})

		It("should not overwrite the nginxIngress field for shoot with workers", func() {
			s := corev1.ServiceExternalTrafficPolicyLocal
			obj.Spec.Addons = &Addons{}
			obj.Spec.Addons.NginxIngress = &NginxIngress{ExternalTrafficPolicy: &s}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons.NginxIngress).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).To(PointTo(Equal(corev1.ServiceExternalTrafficPolicyLocal)))
		})

		It("should not set the addons field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil
			obj.Spec.Addons = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons).To(BeNil())
		})
	})

	Describe("Maintenance defaulting", func() {
		BeforeEach(func() {
			obj.Spec.Maintenance = nil
		})

		It("should correctly default the maintenance timeWindow field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance.TimeWindow).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.TimeWindow.Begin).To(HaveSuffix("0000+0000"))
			Expect(obj.Spec.Maintenance.TimeWindow.End).To(HaveSuffix("0000+0000"))
		})

		It("should set the maintenance autoUpdate field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
		})

		It("should set both KubernetesVersion and MachineImageVersion field for shoot with workers", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
			Expect(obj.Spec.Maintenance.AutoUpdate.MachineImageVersion).NotTo(BeNil())
		})

		It("should set only KubernetesVersion field for workerless shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
			Expect(obj.Spec.Maintenance.AutoUpdate.MachineImageVersion).To(BeNil())
		})
	})

	Describe("KubeAPIServer defaulting", func() {
		BeforeEach(func() {
			obj.Spec.Kubernetes.KubeAPIServer = nil
		})

		It("should default API server requests fields", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests).NotTo(BeNil())
		})

		It("should default the max inflight requests fields", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight).To(Equal(pointer.Int32(400)))
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight).To(Equal(pointer.Int32(200)))
		})

		It("should not default the max inflight requests fields if it is already set", func() {
			var (
				maxNonMutatingRequestsInflight int32 = 123
				maxMutatingRequestsInflight    int32 = 456
			)

			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{Requests: &APIServerRequests{}}
			obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight = &maxNonMutatingRequestsInflight
			obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight = &maxMutatingRequestsInflight

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight).To(Equal(&maxNonMutatingRequestsInflight))
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight).To(Equal(&maxMutatingRequestsInflight))
		})

		It("should disable anonymous authentication by default", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication).To(PointTo(BeFalse()))
		})

		It("should not default the anonymous authentication field if it is explicitly set", func() {
			trueVar := true
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{EnableAnonymousAuthentication: &trueVar}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication).To(PointTo(BeTrue()))
		})

		It("should default the event ttl field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EventTTL).To(Equal(&metav1.Duration{Duration: time.Hour}))
		})

		It("should not default the event ttl field if it is already set", func() {
			eventTTL := &metav1.Duration{Duration: 4 * time.Hour}
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{EventTTL: eventTTL}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EventTTL).To(Equal(eventTTL))
		})

		It("should default the log verbosity level", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Logging.Verbosity).To(PointTo(Equal(int32(2))))
		})

		It("should not overwrite the log verbosity level", func() {
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{Logging: &APIServerLogging{Verbosity: pointer.Int32(3)}}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Logging.Verbosity).To(PointTo(Equal(int32(3))))
		})

		It("should not default the access log level", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Logging.HTTPAccessVerbosity).To(BeNil())
		})

		It("should default the defaultNotReadyTolerationSeconds field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds).To(PointTo(Equal(int64(300))))
		})

		It("should not default the defaultNotReadyTolerationSeconds field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds).To(BeNil())
		})

		It("should not overwrite the defaultNotReadyTolerationSeconds field if it is already set", func() {
			var tolerationSeconds int64 = 120
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{DefaultNotReadyTolerationSeconds: pointer.Int64(tolerationSeconds)}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds).To(PointTo(Equal(tolerationSeconds)))
		})

		It("should default the defaultUnreachableTolerationSeconds field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds).To(PointTo(Equal(int64(300))))
		})

		It("should not default the defaultUnreachableTolerationSeconds field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds).To(BeNil())
		})

		It("should not overwrite the defaultUnreachableTolerationSeconds field if it is already set", func() {
			var tolerationSeconds int64 = 120
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{DefaultUnreachableTolerationSeconds: pointer.Int64(tolerationSeconds)}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds).To(PointTo(Equal(tolerationSeconds)))
		})
	})

	It("should default architecture of worker's machine to amd64", func() {
		obj.Spec.Provider.Workers = []Worker{
			{Name: "Default Worker"},
			{Name: "Worker with machine architecture type",
				Machine: Machine{Architecture: pointer.String("test")}},
		}

		SetObjectDefaults_Shoot(obj)

		Expect(*obj.Spec.Provider.Workers[0].Machine.Architecture).To(Equal(v1beta1constants.ArchitectureAMD64))
		Expect(*obj.Spec.Provider.Workers[1].Machine.Architecture).To(Equal("test"))
	})

	It("should default worker cri.name to containerd", func() {
		obj.Spec.Provider.Workers = []Worker{
			{Name: "DefaultWorker"},
			{Name: "Worker with CRI configuration",
				CRI: &CRI{Name: "some configured value"}},
		}

		SetObjectDefaults_Shoot(obj)

		Expect(obj.Spec.Provider.Workers[0].CRI).ToNot(BeNil())
		Expect(obj.Spec.Provider.Workers[0].CRI.Name).To(Equal(CRINameContainerD))
		Expect(obj.Spec.Provider.Workers[1].CRI.Name).To(BeEquivalentTo("some configured value"))
	})

	It("should set the workers settings field for shoot with workers", func() {
		obj.Spec.Provider.WorkersSettings = nil

		SetObjectDefaults_Shoot(obj)

		Expect(obj.Spec.Provider.WorkersSettings).To(Equal(&WorkersSettings{SSHAccess: &SSHAccess{Enabled: true}}))
	})

	It("should not set the workers settings field for workerless Shoot", func() {
		obj.Spec.Provider.Workers = nil

		SetObjectDefaults_Shoot(obj)

		Expect(obj.Spec.Provider.WorkersSettings).To(BeNil())
	})

	It("should not overwrite the ssh access field in workers settings", func() {
		obj.Spec.Provider.WorkersSettings = &WorkersSettings{
			SSHAccess: &SSHAccess{
				Enabled: false,
			},
		}

		SetObjectDefaults_Shoot(obj)

		Expect(obj.Spec.Provider.WorkersSettings).To(Equal(&WorkersSettings{SSHAccess: &SSHAccess{Enabled: false}}))
	})

	Describe("SystemComponents defaulting", func() {
		It("should set the system components and coredns autoscaling fields for shoot with workers", func() {
			obj.Spec.SystemComponents = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SystemComponents).To(Equal(&SystemComponents{CoreDNS: &CoreDNS{Autoscaling: &CoreDNSAutoscaling{Mode: CoreDNSAutoscalingModeHorizontal}}}))
		})

		It("should not set the system components for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SystemComponents).To(BeNil())
		})
	})

	Context("K8s version >= 1.25", func() {
		BeforeEach(func() {
			obj.Spec.Kubernetes.Version = "1.25.0"
		})

		Context("allowPrivilegedContainers field is not set", func() {
			It("should not set the field", func() {
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.AllowPrivilegedContainers).To(BeNil())
			})
		})
	})

	Describe("Worker defaulting", func() {
		It("should set the maxSurge field", func() {
			SetObjectDefaults_Shoot(obj)

			for i := range obj.Spec.Provider.Workers {
				a := &obj.Spec.Provider.Workers[i]
				Expect(a.MaxSurge).To(PointTo(Equal(intstr.FromInt32(1))))
			}
		})

		It("should set the maxUnavailable field", func() {
			SetObjectDefaults_Shoot(obj)

			for i := range obj.Spec.Provider.Workers {
				a := &obj.Spec.Provider.Workers[i]
				Expect(a.MaxUnavailable).To(PointTo(Equal(intstr.FromInt32(0))))
			}
		})

		It("should set the allowSystemComponents field", func() {
			SetObjectDefaults_Shoot(obj)

			for i := range obj.Spec.Provider.Workers {
				a := &obj.Spec.Provider.Workers[i]
				Expect(a.SystemComponents.Allow).To(BeTrue())
			}
		})
	})

	Describe("ClusterAutoscaler defaulting", func() {
		var (
			expanderRandom     = ClusterAutoscalerExpanderRandom
			expanderLeastWaste = ClusterAutoscalerExpanderLeastWaste
		)

		It("should correctly default the ClusterAutoscaler", func() {
			obj.Spec.Kubernetes.ClusterAutoscaler = &ClusterAutoscaler{}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterAdd).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterDelete).To(PointTo(Equal(metav1.Duration{Duration: 0})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterFailure).To(PointTo(Equal(metav1.Duration{Duration: 3 * time.Minute})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUnneededTime).To(PointTo(Equal(metav1.Duration{Duration: 30 * time.Minute})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScanInterval).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Second})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxNodeProvisionTime).To(PointTo(Equal(metav1.Duration{Duration: 20 * time.Minute})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.Expander).To(PointTo(Equal(expanderLeastWaste)))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxGracefulTerminationSeconds).To(PointTo(Equal(int32(600))))
		})

		It("should correctly override values the ClusterAutoscaler on setting them", func() {
			obj.Spec.Kubernetes.ClusterAutoscaler = &ClusterAutoscaler{
				ScaleDownDelayAfterAdd:        &metav1.Duration{Duration: 1 * time.Hour},
				ScaleDownDelayAfterDelete:     &metav1.Duration{Duration: 2 * time.Hour},
				ScaleDownDelayAfterFailure:    &metav1.Duration{Duration: 3 * time.Hour},
				ScaleDownUnneededTime:         &metav1.Duration{Duration: 4 * time.Hour},
				ScaleDownUtilizationThreshold: pointer.Float64(0.8),
				ScanInterval:                  &metav1.Duration{Duration: 5 * time.Hour},
				Expander:                      &expanderRandom,
				MaxNodeProvisionTime:          &metav1.Duration{Duration: 6 * time.Hour},
				MaxGracefulTerminationSeconds: pointer.Int32(60 * 60 * 24),
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterAdd).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterDelete).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterFailure).To(PointTo(Equal(metav1.Duration{Duration: 3 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUnneededTime).To(PointTo(Equal(metav1.Duration{Duration: 4 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScanInterval).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxNodeProvisionTime).To(PointTo(Equal(metav1.Duration{Duration: 6 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.Expander).To(PointTo(Equal(ClusterAutoscalerExpanderRandom)))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxGracefulTerminationSeconds).To(PointTo(Equal(int32(60 * 60 * 24))))
		})
	})

	Describe("VerticalPodAutoscaler defaulting", func() {
		It("should correctly default the VerticalPodAutoscaler", func() {
			obj.Spec.Kubernetes.VerticalPodAutoscaler = &VerticalPodAutoscaler{}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.Enabled).To(BeFalse())
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictAfterOOMThreshold).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Minute})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictionRateBurst).To(PointTo(Equal(int32(1))))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictionRateLimit).To(PointTo(Equal(float64(-1))))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictionTolerance).To(PointTo(Equal(0.5)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationMarginFraction).To(PointTo(Equal(0.15)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.UpdaterInterval).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommenderInterval).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})
	})
})
