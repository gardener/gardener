// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("Shoot defaulting", func() {
	var obj *Shoot

	BeforeEach(func() {
		obj = &Shoot{
			Spec: ShootSpec{
				Kubernetes: Kubernetes{
					Version: "1.31.1",
				},
				Provider: Provider{
					Workers: []Worker{{}},
				},
			},
		}
	})

	Describe("Kubernetes defaulting", func() {
		Describe("KubeControllerManager settings defaulting", func() {
			Describe("KubeControllerManager", func() {
				It("should not default KubeControllerManager field for workerless Shoot", func() {
					obj.Spec.Provider.Workers = nil

					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.KubeControllerManager).To(BeNil())
				})
			})

			Describe("NodeCIDRMaskSize", func() {
				Context("IPv4", func() {
					It("should make nodeCIDRMaskSize big enough for 2*maxPods", func() {
						obj.Spec.Provider.Workers = []Worker{{}}
						obj.Spec.Kubernetes.Kubelet = &KubeletConfig{
							MaxPods: ptr.To[int32](250),
						}

						SetObjectDefaults_Shoot(obj)

						Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(ptr.To[int32](23)))
					})

					It("should make nodeCIDRMaskSize big enough for 2*maxPods (consider worker pool settings)", func() {
						obj.Spec.Kubernetes.Kubelet = &KubeletConfig{
							MaxPods: ptr.To[int32](64),
						}
						obj.Spec.Provider.Workers = []Worker{{
							Name: "1",
							Kubernetes: &WorkerKubernetes{
								Kubelet: &KubeletConfig{
									MaxPods: ptr.To[int32](100),
								},
							},
						}, {
							Name: "2",
							Kubernetes: &WorkerKubernetes{
								Kubelet: &KubeletConfig{
									MaxPods: ptr.To[int32](260),
								},
							},
						}}

						SetObjectDefaults_Shoot(obj)

						Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(Equal(ptr.To[int32](22)))
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

						Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize).To(PointTo(Equal(int32(64))))
					})
				})
			})

			Describe("NodeMonitorGracePeriod", func() {
				It("should default the nodeMonitorGracePeriod field", func() {
					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod).To(PointTo(Equal(metav1.Duration{Duration: 40 * time.Second})))
				})

				It("should not overwrite the already set values for nodeMonitorGracePeriod field", func() {
					duration := metav1.Duration{Duration: 2 * time.Minute}
					obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{
						NodeMonitorGracePeriod: &duration,
					}

					SetObjectDefaults_Shoot(obj)

					Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod).To(PointTo(Equal(duration)))
				})
			})
		})

		Describe("KubeScheduler defaulting", func() {
			It("should not default the kubeScheduler field for workerless Shoot", func() {
				obj.Spec.Provider.Workers = nil
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeScheduler).To(BeNil())
			})

			It("should default the kubeScheduler field", func() {
				obj.Spec.Kubernetes.KubeScheduler = &KubeSchedulerConfig{}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeScheduler.Profile).To(PointTo(Equal(SchedulingProfileBalanced)))
			})

			It("should not overwrite the already set values for kubeScheduler profile field", func() {
				profile := SchedulingProfileBinPacking
				obj.Spec.Kubernetes.KubeScheduler = &KubeSchedulerConfig{
					Profile: &profile,
				}

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeScheduler.Profile).To(PointTo(Equal(SchedulingProfileBinPacking)))
			})
		})

		Describe("KubeProxy defaulting", func() {
			It("should not default the kubeProxy field for workerless Shoot", func() {
				obj.Spec.Provider.Workers = nil
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeProxy).To(BeNil())
			})

			It("should default the kubeProxy field for Shoot with workers", func() {
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeProxy).NotTo(BeNil())
				Expect(obj.Spec.Kubernetes.KubeProxy.Mode).To(PointTo(Equal(ProxyModeIPTables)))
				Expect(obj.Spec.Kubernetes.KubeProxy.Enabled).To(PointTo(BeTrue()))
			})

			It("should not overwrite the already set values for kubeProxy field", func() {
				mode := ProxyModeIPVS
				obj.Spec.Kubernetes.KubeProxy = &KubeProxyConfig{
					Mode:    &mode,
					Enabled: ptr.To(false),
				}
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.KubeProxy).NotTo(BeNil())
				Expect(obj.Spec.Kubernetes.KubeProxy.Mode).To(PointTo(Equal(ProxyModeIPVS)))
				Expect(obj.Spec.Kubernetes.KubeProxy.Enabled).To(PointTo(BeFalse()))
			})
		})

		Describe("Kubelet defaulting", func() {
			BeforeEach(func() {
				obj.Spec.Kubernetes.Kubelet = nil
			})

			It("should not default the kubelet field for workerless Shoot", func() {
				obj.Spec.Provider.Workers = nil
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet).To(BeNil())
			})

			It("should default the kubelet field for Shoot with workers", func() {
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet).NotTo(BeNil())
			})

			It("should default the failSwapOn field", func() {
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeTrue()))
			})

			It("should not overwrite already set values for failSwapOn field", func() {
				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.FailSwapOn = ptr.To(false)

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeFalse()))
			})

			It("should default the swap behaviour", func() {
				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.FailSwapOn = ptr.To(false)
				obj.Spec.Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap).To(Not(BeNil()))
				Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(PointTo(Equal(LimitedSwap)))
			})

			It("should not overwrite already set values for swap behaviour", func() {
				unlimitedSwap := UnlimitedSwap
				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.FailSwapOn = ptr.To(false)
				obj.Spec.Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
				obj.Spec.Kubernetes.Kubelet.MemorySwap = &MemorySwapConfiguration{SwapBehavior: &unlimitedSwap}
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap).To(Not(BeNil()))
				Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(PointTo(Equal(UnlimitedSwap)))
			})

			It("should not default the swap behaviour because failSwapOn=true", func() {
				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.FailSwapOn = ptr.To(true)
				obj.Spec.Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap).To(BeNil())
			})

			It("should not default the swap behaviour because kubelet NodeSwap feature gate is false", func() {
				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.FailSwapOn = ptr.To(false)
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.MemorySwap).To(BeNil())
			})

			It("should default the imageGCThreshold fields", func() {
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent).To(PointTo(Equal(int32(50))))
				Expect(obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent).To(PointTo(Equal(int32(40))))
			})

			It("should not overwrite already set values for imageGCThreshold field", func() {
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

			It("should default serializeImagePulls to true when maxParallelImagePulls is unset or set to less than 2", func() {
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.ImageMinimumGCAge).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Minute})))
				Expect(obj.Spec.Kubernetes.Kubelet.ImageMaximumGCAge).To(PointTo(Equal(metav1.Duration{})))
			})

			It("should not overwrite already set values for imageGC age fields", func() {
				var (
					minAge = metav1.Duration{Duration: 5 * time.Minute}
					maxAge = metav1.Duration{Duration: 10 * time.Minute}
				)

				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.ImageMinimumGCAge = &minAge
				obj.Spec.Kubernetes.Kubelet.ImageMaximumGCAge = &maxAge

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.ImageMinimumGCAge).To(PointTo(Equal(minAge)))
				Expect(obj.Spec.Kubernetes.Kubelet.ImageMaximumGCAge).To(PointTo(Equal(maxAge)))
			})

			It("should default the serializeImagePulls field", func() {
				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.SerializeImagePulls).To(PointTo(BeTrue()))
			})

			It("should not overwrite already set values for serializeImagePulls field", func() {
				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.SerializeImagePulls = ptr.To(false)

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.SerializeImagePulls).To(PointTo(BeFalse()))
			})

			It("should not overwrite already set values for maxParallelImagePulls field", func() {
				var limitParallelImagePulls int32 = 5

				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.MaxParallelImagePulls = &limitParallelImagePulls

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.MaxParallelImagePulls).To(PointTo(Equal(int32(5))))
			})

			It("should default serializeImagePulls to false when maxParallelImagePulls field is set", func() {
				var limitParallelImagePulls int32 = 5

				obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
				obj.Spec.Kubernetes.Kubelet.MaxParallelImagePulls = &limitParallelImagePulls

				SetObjectDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.MaxParallelImagePulls).To(PointTo(Equal(int32(5))))
				Expect(obj.Spec.Kubernetes.Kubelet.SerializeImagePulls).To(PointTo(Equal(false)))
			})
		})
	})

	Describe("Worker Swap", func() {
		It("should default the swap behaviour for a worker pool", func() {
			obj.Spec.Provider.Workers = []Worker{
				{
					Kubernetes: &WorkerKubernetes{
						Kubelet: &KubeletConfig{},
					},
				},
			}
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FailSwapOn = ptr.To(false)
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap).To(Not(BeNil()))
			Expect(obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(PointTo(Equal(LimitedSwap)))
		})

		It("should not overwrite already set values for swap behaviour for a worker pool", func() {
			unlimitedSwap := UnlimitedSwap
			obj.Spec.Provider.Workers = []Worker{
				{
					Kubernetes: &WorkerKubernetes{
						Kubelet: &KubeletConfig{
							MemorySwap: &MemorySwapConfiguration{
								SwapBehavior: &unlimitedSwap,
							},
						},
					},
				},
			}
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FailSwapOn = ptr.To(false)
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FeatureGates = map[string]bool{"NodeSwap": true}
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(PointTo(Equal(UnlimitedSwap)))
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
			obj.Spec.Provider.Workers = []Worker{
				{
					Kubernetes: &WorkerKubernetes{
						Kubelet: &KubeletConfig{},
					},
				},
			}
			obj.Spec.Provider.Workers[0].Kubernetes.Kubelet.FailSwapOn = ptr.To(false)
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

		It("should not overwrite the already set values for schedulerName field", func() {
			obj.Spec.SchedulerName = ptr.To("foo-scheduler")

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SchedulerName).To(PointTo(Equal("foo-scheduler")))
		})
	})

	Describe("Networking", func() {
		BeforeEach(func() {
			obj.Spec.Networking = nil
		})

		It("should default the networking field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Networking).NotTo(BeNil())
		})

		It("should default ipFamilies setting to IPv4 single-stack", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Networking).NotTo(BeNil())
			Expect(obj.Spec.Networking.IPFamilies).To(ConsistOf(IPFamilyIPv4))
		})

		It("should not overwrite already set values for ipFamilies value", func() {
			obj.Spec.Networking = &Networking{}
			obj.Spec.Networking.IPFamilies = []IPFamily{IPFamilyIPv6}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Networking).NotTo(BeNil())
			Expect(obj.Spec.Networking.IPFamilies).To(ConsistOf(IPFamilyIPv6))
		})
	})

	Describe("Addons defaulting", func() {
		It("should default the addons field for shoot with workers", func() {
			obj.Spec.Addons = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons).NotTo(BeNil())
		})

		It("should default the kubernetesDashboard field for shoot with workers", func() {
			obj.Spec.Addons = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons).NotTo(BeNil())
			Expect(obj.Spec.Addons.KubernetesDashboard).NotTo(BeNil())
			Expect(obj.Spec.Addons.KubernetesDashboard.AuthenticationMode).To(PointTo(Equal(KubernetesDashboardAuthModeToken)))
		})

		It("should not overwrite the already set values for kubernetesDashboard field", func() {
			obj.Spec.Addons = &Addons{
				KubernetesDashboard: &KubernetesDashboard{
					AuthenticationMode: ptr.To("foo"),
				},
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons).NotTo(BeNil())
			Expect(obj.Spec.Addons.KubernetesDashboard).NotTo(BeNil())
			Expect(obj.Spec.Addons.KubernetesDashboard.AuthenticationMode).To(PointTo(Equal("foo")))
		})

		It("should default the nginxIngress field for shoot with workers", func() {
			obj.Spec.Addons = &Addons{}
			obj.Spec.Addons.NginxIngress = &NginxIngress{}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons.NginxIngress).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).To(PointTo(Equal(corev1.ServiceExternalTrafficPolicyCluster)))
		})

		It("should not overwrite the already set values for nginxIngress field", func() {
			s := corev1.ServiceExternalTrafficPolicyLocal
			obj.Spec.Addons = &Addons{}
			obj.Spec.Addons.NginxIngress = &NginxIngress{ExternalTrafficPolicy: &s}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Addons.NginxIngress).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).NotTo(BeNil())
			Expect(obj.Spec.Addons.NginxIngress.ExternalTrafficPolicy).To(PointTo(Equal(corev1.ServiceExternalTrafficPolicyLocal)))
		})

		It("should not default the addons field for workerless Shoot", func() {
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

		It("should default the maintenance timeWindow field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance.TimeWindow).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.TimeWindow.Begin).To(HaveSuffix("0000+0000"))
			Expect(obj.Spec.Maintenance.TimeWindow.End).To(HaveSuffix("0000+0000"))
		})

		It("should default both KubernetesVersion and MachineImageVersion field for shoot with workers", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
			Expect(obj.Spec.Maintenance.AutoUpdate.MachineImageVersion).NotTo(BeNil())
		})

		It("should default only KubernetesVersion field for workerless shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeTrue())
			Expect(obj.Spec.Maintenance.AutoUpdate.MachineImageVersion).To(BeNil())
		})

		It("should not overwrite the already set values for maintenance autoUpdate field", func() {
			obj.Spec.Maintenance = &Maintenance{
				AutoUpdate: &MaintenanceAutoUpdate{
					KubernetesVersion:   false,
					MachineImageVersion: ptr.To(false),
				},
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate).NotTo(BeNil())
			Expect(obj.Spec.Maintenance.AutoUpdate.KubernetesVersion).To(BeFalse())
			Expect(obj.Spec.Maintenance.AutoUpdate.MachineImageVersion).To(PointTo(BeFalse()))
		})
	})

	Describe("KubeAPIServer defaulting", func() {
		BeforeEach(func() {
			obj.Spec.Kubernetes.KubeAPIServer = nil
		})

		It("should default the API server requests fields", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests).NotTo(BeNil())
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight).To(PointTo(Equal(int32(400))))
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight).To(PointTo(Equal(int32(200))))
		})

		It("should not overwrite the already set values for API server requests fields", func() {
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

		It("should default anonymous authentication field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication).To(PointTo(BeFalse()))
		})

		It("should not overwrite the already set values for anonymous authentication field", func() {
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{EnableAnonymousAuthentication: ptr.To(true)}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication).To(PointTo(BeTrue()))
		})

		It("should default the event ttl field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EventTTL).To(Equal(&metav1.Duration{Duration: time.Hour}))
		})

		It("should not overwrite the already set values for event ttl field", func() {
			eventTTL := &metav1.Duration{Duration: 4 * time.Hour}
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{EventTTL: eventTTL}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.EventTTL).To(Equal(eventTTL))
		})

		It("should default the log verbosity level", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Logging.Verbosity).To(PointTo(Equal(int32(2))))
		})

		It("should not overwrite the already set values for log verbosity level", func() {
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{Logging: &APIServerLogging{Verbosity: ptr.To[int32](3)}}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.Logging.Verbosity).To(PointTo(Equal(int32(3))))
		})

		It("should default the defaultNotReadyTolerationSeconds field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds).To(PointTo(Equal(int64(300))))
		})

		It("should not overwrite the already set values for defaultNotReadyTolerationSeconds field", func() {
			var tolerationSeconds int64 = 120
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{DefaultNotReadyTolerationSeconds: ptr.To(tolerationSeconds)}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds).To(PointTo(Equal(tolerationSeconds)))
		})

		It("should not default the defaultNotReadyTolerationSeconds field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds).To(BeNil())
		})

		It("should default the defaultUnreachableTolerationSeconds field", func() {
			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds).To(PointTo(Equal(int64(300))))
		})

		It("should not overwrite the already set values for defaultUnreachableTolerationSeconds field", func() {
			var tolerationSeconds int64 = 120
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{DefaultUnreachableTolerationSeconds: ptr.To(tolerationSeconds)}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds).To(PointTo(Equal(tolerationSeconds)))
		})

		It("should not default the defaultUnreachableTolerationSeconds field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds).To(BeNil())
		})
	})

	Describe("WorkerSettings defaulting", func() {
		It("should default the workers settings field for shoot with workers", func() {
			obj.Spec.Provider.WorkersSettings = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.WorkersSettings).To(Equal(&WorkersSettings{SSHAccess: &SSHAccess{Enabled: true}}))
		})

		It("should not default the workers settings field for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.WorkersSettings).To(BeNil())
		})

		It("should not overwrite the already set values for ssh access field in workers settings", func() {
			obj.Spec.Provider.WorkersSettings = &WorkersSettings{
				SSHAccess: &SSHAccess{
					Enabled: false,
				},
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Provider.WorkersSettings).To(Equal(&WorkersSettings{SSHAccess: &SSHAccess{Enabled: false}}))
		})
	})

	It("should default architecture of worker's machine to amd64", func() {
		obj.Spec.Provider.Workers = []Worker{
			{Name: "Default Worker"},
			{Name: "Worker with machine architecture type",
				Machine: Machine{Architecture: ptr.To("test")}},
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

	Describe("SystemComponents defaulting", func() {
		It("should default the system components and coredns autoscaling fields for shoot with workers", func() {
			obj.Spec.SystemComponents = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SystemComponents).To(Equal(&SystemComponents{CoreDNS: &CoreDNS{Autoscaling: &CoreDNSAutoscaling{Mode: CoreDNSAutoscalingModeHorizontal}}}))
		})

		It("should not overwrite coredns autoscaling fields when mode is set to horizontal for shoot with workers", func() {
			obj.Spec.SystemComponents = &SystemComponents{
				CoreDNS: &CoreDNS{Autoscaling: &CoreDNSAutoscaling{Mode: CoreDNSAutoscalingModeHorizontal}},
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SystemComponents.CoreDNS.Autoscaling.Mode).To(Equal(CoreDNSAutoscalingModeHorizontal))
		})

		It("should not overwrite coredns autoscaling fields when mode is set to cluster-proportional for shoot with workers", func() {
			obj.Spec.SystemComponents = &SystemComponents{
				CoreDNS: &CoreDNS{Autoscaling: &CoreDNSAutoscaling{Mode: CoreDNSAutoscalingModeClusterProportional}},
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SystemComponents.CoreDNS.Autoscaling.Mode).To(Equal(CoreDNSAutoscalingModeClusterProportional))
		})

		It("should overwrite coredns autoscaling mode field to horizontal when mode is not horizontal or cluster-proportional for shoot with workers", func() {
			obj.Spec.SystemComponents = &SystemComponents{
				CoreDNS: &CoreDNS{Autoscaling: &CoreDNSAutoscaling{Mode: "foo"}},
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SystemComponents.CoreDNS.Autoscaling.Mode).To(Equal(CoreDNSAutoscalingModeHorizontal))
		})

		It("should not default the system components for workerless Shoot", func() {
			obj.Spec.Provider.Workers = nil

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.SystemComponents).To(BeNil())
		})
	})

	Describe("Worker defaulting", func() {
		var (
			maxSurge       = intstr.FromInt32(2)
			maxUnavailable = intstr.FromInt32(1)
		)
		It("should default the worker fields", func() {
			SetObjectDefaults_Shoot(obj)

			for _, worker := range obj.Spec.Provider.Workers {
				Expect(worker.MaxSurge).To(PointTo(Equal(intstr.FromInt32(1))))
				Expect(worker.MaxUnavailable).To(PointTo(Equal(intstr.FromInt32(0))))
				Expect(worker.SystemComponents.Allow).To(BeTrue())
				Expect(worker.UpdateStrategy).To(PointTo(Equal(AutoRollingUpdate)))
				Expect(worker.MachineControllerManagerSettings).To(BeNil())
			}
		})

		It("should not overwrite the already set values for worker fields", func() {
			obj.Spec.Provider.Workers = []Worker{
				{
					MaxSurge:         &maxSurge,
					MaxUnavailable:   &maxUnavailable,
					SystemComponents: &WorkerSystemComponents{Allow: false},
					UpdateStrategy:   ptr.To(AutoInPlaceUpdate),
				},
			}

			SetObjectDefaults_Shoot(obj)

			for _, worker := range obj.Spec.Provider.Workers {
				Expect(worker.MaxSurge).To(PointTo(Equal(intstr.FromInt32(2))))
				Expect(worker.MaxUnavailable).To(PointTo(Equal(intstr.FromInt32(1))))
				Expect(worker.SystemComponents.Allow).To(BeFalse())
				Expect(worker.UpdateStrategy).To(PointTo(Equal(AutoInPlaceUpdate)))
			}
		})

		It("should default the worker MachineControllerManagerSettings field also if update strategy is auto in-place update", func() {
			obj.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(AutoInPlaceUpdate)
			SetObjectDefaults_Shoot(obj)

			for _, worker := range obj.Spec.Provider.Workers {
				Expect(worker.MaxSurge).To(PointTo(Equal(intstr.FromInt32(1))))
				Expect(worker.MaxUnavailable).To(PointTo(Equal(intstr.FromInt32(0))))
				Expect(worker.SystemComponents.Allow).To(BeTrue())
				Expect(worker.UpdateStrategy).To(PointTo(Equal(AutoInPlaceUpdate)))
				Expect(worker.MachineControllerManagerSettings).To(Not(BeNil()))
				Expect(worker.MachineControllerManagerSettings.DisableHealthTimeout).To(PointTo(Equal(true)))
			}
		})

		It("should default the worker MachineControllerManagerSettings field also if update strategy is manual in-place update", func() {
			obj.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(ManualInPlaceUpdate)
			SetObjectDefaults_Shoot(obj)

			for _, worker := range obj.Spec.Provider.Workers {
				Expect(worker.MaxSurge).To(PointTo(Equal(intstr.FromInt32(0))))
				Expect(worker.MaxUnavailable).To(PointTo(Equal(intstr.FromInt32(1))))
				Expect(worker.SystemComponents.Allow).To(BeTrue())
				Expect(worker.UpdateStrategy).To(PointTo(Equal(ManualInPlaceUpdate)))
				Expect(worker.MachineControllerManagerSettings).To(Not(BeNil()))
				Expect(worker.MachineControllerManagerSettings.DisableHealthTimeout).To(PointTo(Equal(true)))
			}
		})

		It("should not overwrite the already set worker MachineControllerManagerSettings field", func() {
			obj.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(AutoInPlaceUpdate)
			obj.Spec.Provider.Workers[0].MachineControllerManagerSettings = &MachineControllerManagerSettings{DisableHealthTimeout: ptr.To(false)}
			SetObjectDefaults_Shoot(obj)

			for _, worker := range obj.Spec.Provider.Workers {
				Expect(worker.MaxSurge).To(PointTo(Equal(intstr.FromInt32(1))))
				Expect(worker.MaxUnavailable).To(PointTo(Equal(intstr.FromInt32(0))))
				Expect(worker.SystemComponents.Allow).To(BeTrue())
				Expect(worker.UpdateStrategy).To(PointTo(Equal(AutoInPlaceUpdate)))
				Expect(worker.MachineControllerManagerSettings).To(Not(BeNil()))
				Expect(worker.MachineControllerManagerSettings.DisableHealthTimeout).To(PointTo(Equal(false)))
			}
		})
	})

	Describe("ClusterAutoscaler defaulting", func() {
		var (
			expanderRandom     = ClusterAutoscalerExpanderRandom
			expanderLeastWaste = ClusterAutoscalerExpanderLeastWaste
		)

		It("should default the ClusterAutoscaler field", func() {
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
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.IgnoreDaemonsetsUtilization).To(PointTo(Equal(false)))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.Verbosity).To(PointTo(Equal(int32(2))))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.NewPodScaleUpDelay).To(PointTo(Equal(metav1.Duration{Duration: 0})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxEmptyBulkDelete).To(PointTo(Equal(int32(10))))
		})

		It("should not overwrite the already set values for ClusterAutoscaler field", func() {
			obj.Spec.Kubernetes.ClusterAutoscaler = &ClusterAutoscaler{
				ScaleDownDelayAfterAdd:        &metav1.Duration{Duration: 1 * time.Hour},
				ScaleDownDelayAfterDelete:     &metav1.Duration{Duration: 2 * time.Hour},
				ScaleDownDelayAfterFailure:    &metav1.Duration{Duration: 3 * time.Hour},
				ScaleDownUnneededTime:         &metav1.Duration{Duration: 4 * time.Hour},
				ScaleDownUtilizationThreshold: ptr.To(0.8),
				ScanInterval:                  &metav1.Duration{Duration: 5 * time.Hour},
				Expander:                      &expanderRandom,
				MaxNodeProvisionTime:          &metav1.Duration{Duration: 6 * time.Hour},
				MaxGracefulTerminationSeconds: ptr.To(int32(60 * 60 * 24)),
				IgnoreDaemonsetsUtilization:   ptr.To(true),
				Verbosity:                     ptr.To[int32](4),
				NewPodScaleUpDelay:            &metav1.Duration{Duration: 1},
				MaxEmptyBulkDelete:            ptr.To[int32](20),
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterAdd).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterDelete).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterFailure).To(PointTo(Equal(metav1.Duration{Duration: 3 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUnneededTime).To(PointTo(Equal(metav1.Duration{Duration: 4 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUtilizationThreshold).To(PointTo(Equal(0.8)))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.ScanInterval).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxNodeProvisionTime).To(PointTo(Equal(metav1.Duration{Duration: 6 * time.Hour})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.Expander).To(PointTo(Equal(ClusterAutoscalerExpanderRandom)))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxGracefulTerminationSeconds).To(PointTo(Equal(int32(60 * 60 * 24))))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.IgnoreDaemonsetsUtilization).To(PointTo(Equal(true)))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.Verbosity).To(PointTo(Equal(int32(4))))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.NewPodScaleUpDelay).To(PointTo(Equal(metav1.Duration{Duration: 1})))
			Expect(obj.Spec.Kubernetes.ClusterAutoscaler.MaxEmptyBulkDelete).To(PointTo(Equal(int32(20))))
		})
	})

	Describe("VerticalPodAutoscaler defaulting", func() {
		var (
			evictionTolerance            = 0.6
			recommendationMarginFraction = 0.20
		)

		It("should default the VerticalPodAutoscaler field", func() {
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
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.TargetCPUPercentile).To(PointTo(Equal(0.9)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationLowerBoundCPUPercentile).To(PointTo(Equal(0.5)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationUpperBoundCPUPercentile).To(PointTo(Equal(0.95)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.CPUHistogramDecayHalfLife).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.TargetMemoryPercentile).To(PointTo(Equal(0.9)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationLowerBoundMemoryPercentile).To(PointTo(Equal(0.5)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationUpperBoundMemoryPercentile).To(PointTo(Equal(0.95)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.MemoryHistogramDecayHalfLife).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.MemoryAggregationInterval).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.MemoryAggregationIntervalCount).To(PointTo(Equal(int64(8))))
		})

		It("should not overwrite the already set values for VerticalPodAutoscaler field", func() {
			obj.Spec.Kubernetes.VerticalPodAutoscaler = &VerticalPodAutoscaler{
				EvictAfterOOMThreshold:                   &metav1.Duration{Duration: 5 * time.Minute},
				EvictionRateBurst:                        ptr.To[int32](2),
				EvictionRateLimit:                        ptr.To(1.0),
				EvictionTolerance:                        &evictionTolerance,
				RecommendationMarginFraction:             &recommendationMarginFraction,
				UpdaterInterval:                          &metav1.Duration{Duration: 2 * time.Minute},
				RecommenderInterval:                      &metav1.Duration{Duration: 2 * time.Minute},
				TargetCPUPercentile:                      ptr.To(0.333),
				RecommendationLowerBoundCPUPercentile:    ptr.To(0.303),
				RecommendationUpperBoundCPUPercentile:    ptr.To(0.393),
				CPUHistogramDecayHalfLife:                &metav1.Duration{Duration: 2 * time.Minute},
				TargetMemoryPercentile:                   ptr.To(0.444),
				RecommendationLowerBoundMemoryPercentile: ptr.To(0.404),
				RecommendationUpperBoundMemoryPercentile: ptr.To(0.494),
				MemoryHistogramDecayHalfLife:             &metav1.Duration{Duration: 7 * time.Second},
				MemoryAggregationInterval:                &metav1.Duration{Duration: 22 * time.Minute},
				MemoryAggregationIntervalCount:           ptr.To[int64](42),
			}

			SetObjectDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.Enabled).To(BeFalse())
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictAfterOOMThreshold).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictionRateBurst).To(PointTo(Equal(int32(2))))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictionRateLimit).To(PointTo(Equal(float64(1))))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.EvictionTolerance).To(PointTo(Equal(evictionTolerance)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationMarginFraction).To(PointTo(Equal(recommendationMarginFraction)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.UpdaterInterval).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Minute})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommenderInterval).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Minute})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.TargetCPUPercentile).To(PointTo(Equal(0.333)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationLowerBoundCPUPercentile).To(PointTo(Equal(0.303)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationUpperBoundCPUPercentile).To(PointTo(Equal(0.393)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.CPUHistogramDecayHalfLife).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Minute})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.TargetMemoryPercentile).To(PointTo(Equal(0.444)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationLowerBoundMemoryPercentile).To(PointTo(Equal(0.404)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.RecommendationUpperBoundMemoryPercentile).To(PointTo(Equal(0.494)))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.MemoryHistogramDecayHalfLife).To(PointTo(Equal(metav1.Duration{Duration: 7 * time.Second})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.MemoryAggregationInterval).To(PointTo(Equal(metav1.Duration{Duration: 22 * time.Minute})))
			Expect(obj.Spec.Kubernetes.VerticalPodAutoscaler.MemoryAggregationIntervalCount).To(PointTo(Equal(int64(42))))
		})
	})
})
