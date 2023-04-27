// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("Defaults", func() {
	Describe("#SetObjectDefaults_Seed", func() {
		var obj *Seed

		BeforeEach(func() {
			obj = &Seed{}
		})

		It("should default the seed settings (w/o taints)", func() {
			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Settings.DependencyWatchdog).NotTo(BeNil())
			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeTrue())
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.OwnerChecks.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.TopologyAwareRouting.Enabled).To(BeFalse())
		})

		It("should allow taints that were not allowed in version v1.12", func() {
			taints := []SeedTaint{
				{Key: "seed.gardener.cloud/disable-capacity-reservation"},
				{Key: "seed.gardener.cloud/disable-dns"},
				{Key: "seed.gardener.cloud/invisible"},
			}
			obj.Spec.Taints = taints

			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Settings.DependencyWatchdog).NotTo(BeNil())
			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeTrue())
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.OwnerChecks.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.TopologyAwareRouting.Enabled).To(BeFalse())
			Expect(obj.Spec.Taints).To(HaveLen(3))
			Expect(obj.Spec.Taints).To(Equal(taints))
		})

		It("should not default the seed settings because they were provided", func() {
			var (
				dwdWeederEnabled          = false
				dwdProberEnabled          = false
				topologyAwareRouting      = true
				excessCapacityReservation = false
				scheduling                = true
				vpaEnabled                = false
				ownerChecks               = false
			)

			obj.Spec.Settings = &SeedSettings{
				DependencyWatchdog: &SeedSettingDependencyWatchdog{
					Weeder: &SeedSettingDependencyWatchdogWeeder{Enabled: dwdWeederEnabled},
					Prober: &SeedSettingDependencyWatchdogProber{Enabled: dwdProberEnabled},
				},
				TopologyAwareRouting: &SeedSettingTopologyAwareRouting{
					Enabled: topologyAwareRouting,
				},
				ExcessCapacityReservation: &SeedSettingExcessCapacityReservation{Enabled: excessCapacityReservation},
				Scheduling:                &SeedSettingScheduling{Visible: scheduling},
				VerticalPodAutoscaler:     &SeedSettingVerticalPodAutoscaler{Enabled: vpaEnabled},
				OwnerChecks:               &SeedSettingOwnerChecks{Enabled: ownerChecks},
			}

			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Settings.DependencyWatchdog.Weeder.Enabled).To(Equal(dwdWeederEnabled))
			Expect(obj.Spec.Settings.DependencyWatchdog.Prober.Enabled).To(Equal(dwdProberEnabled))
			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(Equal(excessCapacityReservation))
			Expect(obj.Spec.Settings.Scheduling.Visible).To(Equal(scheduling))
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(Equal(vpaEnabled))
			Expect(obj.Spec.Settings.OwnerChecks.Enabled).To(Equal(ownerChecks))
			Expect(obj.Spec.Settings.TopologyAwareRouting.Enabled).To(Equal(topologyAwareRouting))
		})

		It("should default ipFamilies setting to IPv4 single-stack", func() {
			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Networks.IPFamilies).To(ConsistOf(IPFamilyIPv4))
		})
	})

	Describe("#SetObjectDefaults_Shoot", func() {
		var obj *Shoot

		BeforeEach(func() {
			obj = &Shoot{
				Spec: ShootSpec{
					Kubernetes: Kubernetes{
						Version: "1.22.1",
					},
					Provider: Provider{
						Workers: []Worker{Worker{}},
					},
				},
			}
		})

		Describe("Kubernetes", func() {
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
		})

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

		It("should default the failSwapOn field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeTrue()))
		})

		It("should not default the failSwapOn field", func() {
			falseVar := false
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.FailSwapOn = &falseVar

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeFalse()))
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

		Context("default kubeReserved", func() {
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

		Describe("kubeControllerManager settings", func() {
			It("should not overwrite the kube-controller-manager's node monitor grace period", func() {
				nodeMonitorGracePeriod := &metav1.Duration{Duration: time.Minute}
				obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{NodeMonitorGracePeriod: nodeMonitorGracePeriod}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod).To(Equal(nodeMonitorGracePeriod))
			})

			It("should default the kube-controller-manager's node monitor grace period", func() {
				obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod).To(Equal(&metav1.Duration{Duration: 2 * time.Minute}))
			})

			Describe("nodeCIDRMaskSize", func() {
				Context("IPv4", func() {
					It("should make nodeCIDRMaskSize big enough for 2*maxPods", func() {
						obj.Spec.Provider.Workers = []Worker{Worker{}}
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
						obj.Spec.Provider.Workers = []Worker{Worker{}}
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

		Describe("KubeScheduler", func() {
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
			It("should set the networking field for shoot with workers", func() {
				obj.Spec.Provider.Workers = []Worker{Worker{}}
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Networking).NotTo(BeNil())
			})

			It("should not set the networking field for Workerless shoot", func() {
				obj.Spec.Provider.Workers = nil
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Networking).To(BeNil())
			})

			It("should default ipFamilies setting to IPv4 single-stack for shoot with workers", func() {
				obj.Spec.Provider.Workers = []Worker{Worker{}}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Networking).NotTo(BeNil())
				Expect(obj.Spec.Networking.IPFamilies).To(ConsistOf(IPFamilyIPv4))
			})
		})

		Describe("Addons", func() {
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

			It("should not set the addons field for workerless Shoot", func() {
				obj.Spec.Provider.Workers = nil
				obj.Spec.Addons = nil

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Addons).To(BeNil())
			})
		})

		Describe("maintenance", func() {
			It("should set the maintenance field", func() {
				obj.Spec.Maintenance = nil

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Maintenance).NotTo(BeNil())
				Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
			})

			It("should set both KubernetesVersion and MachineImageVersion field for shoot with workers", func() {
				obj.Spec.Maintenance = nil

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Maintenance).NotTo(BeNil())
				Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
				Expect(obj.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
				Expect(obj.Spec.Maintenance.AutoUpdate.MachineImageVersion).NotTo(BeNil())
			})

			It("should set only KubernetesVersion field for workerless shoot", func() {
				obj.Spec.Provider.Workers = nil
				obj.Spec.Maintenance = nil

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Maintenance).NotTo(BeNil())
				Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
				Expect(obj.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
				Expect(obj.Spec.Maintenance.AutoUpdate.MachineImageVersion).To(BeNil())
			})
		})

		It("should default the max inflight requests fields", func() {
			SetObjectDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight).To(Equal(pointer.Int32(400)))
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight).To(Equal(pointer.Int32(200)))
		})

		It("should not default the max inflight requests fields", func() {
			var (
				maxNonMutatingRequestsInflight int32 = 123
				maxMutatingRequestsInflight    int32 = 456
			)

			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{Requests: &KubeAPIServerRequests{}}
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

		It("should not default the event ttl field", func() {
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
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{Logging: &KubeAPIServerLogging{Verbosity: pointer.Int32(3)}}
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

		It("should not overwrite the defaultUnreachableTolerationSeconds field if it is already set", func() {
			var tolerationSeconds int64 = 120
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{DefaultUnreachableTolerationSeconds: pointer.Int64(tolerationSeconds)}

			SetObjectDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds).To(PointTo(Equal(tolerationSeconds)))
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

		It("should default cri.name to containerd when control plane Kubernetes version >= 1.22", func() {
			obj.Spec.Kubernetes.Version = "1.22"
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

		It("should default cri.name to containerd when worker Kubernetes version >= 1.22", func() {
			obj.Spec.Kubernetes.Version = "1.20"
			obj.Spec.Provider.Workers = []Worker{
				{Name: "DefaultWorker",
					Kubernetes: &WorkerKubernetes{Version: pointer.String("1.22")}},
				{Name: "Worker with CRI configuration",
					CRI: &CRI{Name: "some configured value"}},
			}
			SetObjectDefaults_Shoot(obj)
			Expect(obj.Spec.Provider.Workers[0].CRI).ToNot(BeNil())
			Expect(obj.Spec.Provider.Workers[0].CRI.Name).To(Equal(CRINameContainerD))
			Expect(obj.Spec.Provider.Workers[1].CRI.Name).To(BeEquivalentTo("some configured value"))
		})

		It("should not default cri.name to containerd when control plane Kubernetes version < 1.22", func() {
			obj.Spec.Kubernetes.Version = "1.21"
			obj.Spec.Provider.Workers = []Worker{
				{Name: "DefaultWorker"},
				{Name: "Worker with CRI configuration",
					CRI: &CRI{Name: "some configured value"}},
			}
			SetObjectDefaults_Shoot(obj)
			Expect(obj.Spec.Provider.Workers[0].CRI).To(BeNil())
			Expect(obj.Spec.Provider.Workers[1].CRI.Name).To(BeEquivalentTo("some configured value"))
		})

		It("should not default cri.name to containerd when worker Kubernetes version < 1.22", func() {
			obj.Spec.Kubernetes.Version = "1.22"
			obj.Spec.Provider.Workers = []Worker{
				{Name: "DefaultWorker",
					Kubernetes: &WorkerKubernetes{Version: pointer.String("1.21")}},
				{Name: "Worker with CRI configuration",
					CRI: &CRI{Name: "some configured value"}},
			}
			SetObjectDefaults_Shoot(obj)
			Expect(obj.Spec.Provider.Workers[0].CRI).To(BeNil())
			Expect(obj.Spec.Provider.Workers[1].CRI.Name).To(BeEquivalentTo("some configured value"))
		})

		It("should set the workers settings field for shoot with workers", func() {
			obj.Spec.Provider.WorkersSettings = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.WorkersSettings).To(Equal(&WorkersSettings{SSHAccess: &SSHAccess{Enabled: true}}))
		})

		It("should not set the workers settings field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil
			obj.Spec.Provider.WorkersSettings = nil

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

		Describe("SystemComponents", func() {
			It("should set the system components and coredns autoscaling fields for shoot with workers", func() {
				obj.Spec.SystemComponents = nil

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.SystemComponents).To(Equal(&SystemComponents{CoreDNS: &CoreDNS{Autoscaling: &CoreDNSAutoscaling{Mode: CoreDNSAutoscalingModeHorizontal}}}))
			})

			It("should not set the system components for workerless Shoot", func() {
				obj.Spec.Provider.Workers = nil
				obj.Spec.SystemComponents = nil

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.SystemComponents).To(BeNil())
			})
		})

		Context("static token kubeconfig", func() {
			It("should not default the enableStaticTokenKubeconfig field when it is set", func() {
				obj.Spec.Kubernetes = Kubernetes{
					Version:                     "1.24.0",
					EnableStaticTokenKubeconfig: pointer.Bool(false),
				}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(PointTo(BeFalse()))
			})

			It("should default the enableStaticTokenKubeconfig field to true for k8s version < 1.26", func() {
				obj.Spec.Kubernetes = Kubernetes{Version: "1.25.0"}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(PointTo(BeTrue()))
			})

			It("should default the enableStaticTokenKubeconfig field to false for k8s version >= 1.26", func() {
				obj.Spec.Kubernetes = Kubernetes{Version: "1.26.0"}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(PointTo(BeFalse()))
			})
		})

		Context("k8s version < 1.25", func() {
			BeforeEach(func() {
				obj.Spec.Kubernetes = Kubernetes{
					Version:       "1.24.0",
					KubeAPIServer: &KubeAPIServerConfig{},
				}
				obj.Spec.Provider = Provider{
					Workers: []Worker{Worker{}},
				}
			})

			Context("allowPrivilegedContainers field is not set", func() {
				It("should set the field to true if PodSecurityPolicy admission plugin is not disabled and shoot has workers", func() {
					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.AllowPrivilegedContainers).To(PointTo(BeTrue()))
				})

				It("should not set the field if the shoot is workerless", func() {
					obj.Spec.Provider.Workers = nil
					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.AllowPrivilegedContainers).To(BeNil())
				})

				It("should not default the field if PodSecurityPolicy admission plugin is disabled in the shoot spec", func() {
					obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{
						AdmissionPlugins: []AdmissionPlugin{
							{
								Name:     "PodSecurityPolicy",
								Disabled: pointer.Bool(true),
							},
						},
					}
					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.AllowPrivilegedContainers).To(BeNil())
				})

				It("should not default the field if the Shoot is workerless", func() {
					obj.Spec.Provider.Workers = nil
					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.AllowPrivilegedContainers).To(BeNil())
				})
			})

			Context("allowPrivilegedContainers field is set", func() {
				BeforeEach(func() {
					obj.Spec.Kubernetes.AllowPrivilegedContainers = pointer.Bool(false)
				})

				It("should not set the field", func() {
					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.AllowPrivilegedContainers).To(PointTo(BeFalse()))
				})
			})
		})

		Context("k8s version >= 1.25", func() {
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
	})

	Describe("#SetDefaults_Maintenance", func() {
		var obj *Maintenance

		BeforeEach(func() {
			obj = &Maintenance{AutoUpdate: &MaintenanceAutoUpdate{}}
		})

		It("should correctly default the maintenance", func() {
			obj.TimeWindow = nil

			SetDefaults_Maintenance(obj)

			Expect(obj.TimeWindow).NotTo(BeNil())
			Expect(obj.TimeWindow.Begin).To(HaveSuffix("0000+0000"))
			Expect(obj.TimeWindow.End).To(HaveSuffix("0000+0000"))
		})
	})

	Describe("#SetDefaults_Worker", func() {
		var obj *Worker

		BeforeEach(func() {
			obj = &Worker{}
		})

		It("should set the allowSystemComponents field", func() {
			obj.SystemComponents = nil

			SetDefaults_Worker(obj)

			Expect(obj.SystemComponents.Allow).To(BeTrue())
		})
	})

	Describe("#SetDefaults_ClusterAutoscaler", func() {
		var (
			obj                *ClusterAutoscaler
			expanderRandom     = ClusterAutoscalerExpanderRandom
			expanderLeastWaste = ClusterAutoscalerExpanderLeastWaste
		)

		BeforeEach(func() {
			obj = &ClusterAutoscaler{}
		})

		It("should correctly default the ClusterAutoscaler", func() {
			SetDefaults_ClusterAutoscaler(obj)

			Expect(obj.ScaleDownDelayAfterAdd).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
			Expect(obj.ScaleDownDelayAfterDelete).To(PointTo(Equal(metav1.Duration{Duration: 0})))
			Expect(obj.ScaleDownDelayAfterFailure).To(PointTo(Equal(metav1.Duration{Duration: 3 * time.Minute})))
			Expect(obj.ScaleDownUnneededTime).To(PointTo(Equal(metav1.Duration{Duration: 30 * time.Minute})))
			Expect(obj.ScanInterval).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Second})))
			Expect(obj.MaxNodeProvisionTime).To(PointTo(Equal(metav1.Duration{Duration: 20 * time.Minute})))
			Expect(obj.Expander).To(PointTo(Equal(expanderLeastWaste)))
			Expect(obj.MaxGracefulTerminationSeconds).To(PointTo(Equal(int32(600))))
		})

		It("should correctly override values the ClusterAutoscaler on setting them", func() {
			obj = &ClusterAutoscaler{
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

			SetDefaults_ClusterAutoscaler(obj)

			Expect(obj.ScaleDownDelayAfterAdd).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
			Expect(obj.ScaleDownDelayAfterDelete).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Hour})))
			Expect(obj.ScaleDownDelayAfterFailure).To(PointTo(Equal(metav1.Duration{Duration: 3 * time.Hour})))
			Expect(obj.ScaleDownUnneededTime).To(PointTo(Equal(metav1.Duration{Duration: 4 * time.Hour})))
			Expect(obj.ScanInterval).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Hour})))
			Expect(obj.MaxNodeProvisionTime).To(PointTo(Equal(metav1.Duration{Duration: 6 * time.Hour})))
			Expect(obj.Expander).To(PointTo(Equal(ClusterAutoscalerExpanderRandom)))
			Expect(obj.MaxGracefulTerminationSeconds).To(PointTo(Equal(int32(60 * 60 * 24))))
		})
	})

	Describe("#SetDefaults_VerticalPodAutoscaler", func() {
		var obj *VerticalPodAutoscaler

		BeforeEach(func() {
			obj = &VerticalPodAutoscaler{}
		})

		It("should correctly default the VerticalPodAutoscaler", func() {
			SetDefaults_VerticalPodAutoscaler(obj)

			Expect(obj.Enabled).To(BeFalse())
			Expect(obj.EvictAfterOOMThreshold).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Minute})))
			Expect(obj.EvictionRateBurst).To(PointTo(Equal(int32(1))))
			Expect(obj.EvictionRateLimit).To(PointTo(Equal(float64(-1))))
			Expect(obj.EvictionTolerance).To(PointTo(Equal(0.5)))
			Expect(obj.RecommendationMarginFraction).To(PointTo(Equal(0.15)))
			Expect(obj.UpdaterInterval).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			Expect(obj.RecommenderInterval).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})
	})
})
