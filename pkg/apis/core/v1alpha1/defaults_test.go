// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1_test

import (
	"time"

	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_Project", func() {
		var obj *Project

		BeforeEach(func() {
			obj = &Project{}
		})

		Context("api group defaulting", func() {
			DescribeTable(
				"should default the owner api groups",
				func(owner *rbacv1.Subject, kind string, expectedAPIGroup string) {
					if owner != nil {
						owner.Kind = kind
					}

					obj.Spec.Owner = owner

					SetDefaults_Project(obj)

					if owner != nil {
						Expect(obj.Spec.Owner.APIGroup).To(Equal(expectedAPIGroup))
					} else {
						Expect(obj.Spec.Owner).To(BeNil())
					}
				},
				Entry("do nothing because owner is nil", nil, "", ""),
				Entry("kind serviceaccount", &rbacv1.Subject{}, rbacv1.ServiceAccountKind, ""),
				Entry("kind user", &rbacv1.Subject{}, rbacv1.UserKind, rbacv1.GroupName),
				Entry("kind group", &rbacv1.Subject{}, rbacv1.GroupKind, rbacv1.GroupName),
			)

			It("should default the api groups of members", func() {
				member1 := ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "group",
						Kind:     "kind",
						Name:     "member1",
					},
					Roles: []string{"role"},
				}
				member2 := ProjectMember{
					Subject: rbacv1.Subject{
						Kind: rbacv1.ServiceAccountKind,
						Name: "member2",
					},
					Roles: []string{"role"},
				}
				member3 := ProjectMember{
					Subject: rbacv1.Subject{
						Kind: rbacv1.UserKind,
						Name: "member3",
					},
					Roles: []string{"role"},
				}
				member4 := ProjectMember{
					Subject: rbacv1.Subject{
						Kind: rbacv1.GroupKind,
						Name: "member4",
					},
					Roles: []string{"role"},
				}

				obj.Spec.Members = []ProjectMember{member1, member2, member3, member4}

				SetDefaults_Project(obj)

				Expect(obj.Spec.Members[0].APIGroup).To(Equal(member1.Subject.APIGroup))
				Expect(obj.Spec.Members[1].APIGroup).To(BeEmpty())
				Expect(obj.Spec.Members[2].APIGroup).To(Equal(rbacv1.GroupName))
				Expect(obj.Spec.Members[3].APIGroup).To(Equal(rbacv1.GroupName))
			})
		})

		It("should default the roles of members", func() {
			member1 := ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member1",
				},
			}
			member2 := ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member2",
				},
			}

			obj.Spec.Members = []ProjectMember{member1, member2}

			SetDefaults_Project(obj)

			for _, m := range obj.Spec.Members {
				Expect(m.Role).NotTo(HaveLen(0))
				Expect(m.Role).To(Equal(ProjectMemberViewer))
			}
		})

		It("should not add the 'protected' toleration if the namespace is not 'garden' (w/o existing project tolerations)", func() {
			obj.Spec.Namespace = pointer.String("foo")

			SetDefaults_Project(obj)

			Expect(obj.Spec.Tolerations).To(BeNil())
		})

		It("should not add the 'protected' toleration if the namespace is not 'garden' (w/ existing project tolerations)", func() {
			obj.Spec.Namespace = pointer.String("foo")
			obj.Spec.Tolerations = &ProjectTolerations{
				Defaults:  []Toleration{{Key: "foo"}},
				Whitelist: []Toleration{{Key: "bar"}},
			}

			SetDefaults_Project(obj)

			Expect(obj.Spec.Tolerations.Defaults).To(Equal([]Toleration{{Key: "foo"}}))
			Expect(obj.Spec.Tolerations.Whitelist).To(Equal([]Toleration{{Key: "bar"}}))
		})

		It("should add the 'protected' toleration if the namespace is 'garden' (w/o existing project tolerations)", func() {
			obj.Spec.Namespace = pointer.String(v1beta1constants.GardenNamespace)
			obj.Spec.Tolerations = nil

			SetDefaults_Project(obj)

			Expect(obj.Spec.Tolerations.Defaults).To(Equal([]Toleration{{Key: SeedTaintProtected}}))
			Expect(obj.Spec.Tolerations.Whitelist).To(Equal([]Toleration{{Key: SeedTaintProtected}}))
		})

		It("should add the 'protected' toleration if the namespace is 'garden' (w/ existing project tolerations)", func() {
			obj.Spec.Namespace = pointer.String(v1beta1constants.GardenNamespace)
			obj.Spec.Tolerations = &ProjectTolerations{
				Defaults:  []Toleration{{Key: "foo"}},
				Whitelist: []Toleration{{Key: "bar"}},
			}

			SetDefaults_Project(obj)

			Expect(obj.Spec.Tolerations.Defaults).To(Equal([]Toleration{{Key: "foo"}, {Key: SeedTaintProtected}}))
			Expect(obj.Spec.Tolerations.Whitelist).To(Equal([]Toleration{{Key: "bar"}, {Key: SeedTaintProtected}}))
		})
	})

	Describe("#SetDefaults_ControllerResource", func() {
		It("should default the primary field", func() {
			resource := ControllerResource{}

			SetDefaults_ControllerResource(&resource)

			Expect(resource.Primary).To(PointTo(BeTrue()))
		})

		It("should not default the primary field", func() {
			resource := ControllerResource{Primary: pointer.Bool(false)}
			resourceCopy := resource.DeepCopy()

			SetDefaults_ControllerResource(&resource)

			Expect(resource.Primary).To(Equal(resourceCopy.Primary))
		})
	})

	Describe("#SetDefaults_ControllerDeployment", func() {
		var (
			ondemand = ControllerDeploymentPolicyOnDemand
			always   = ControllerDeploymentPolicyAlways
		)

		It("should default the policy field", func() {
			deployment := ControllerRegistrationDeployment{}

			SetDefaults_ControllerRegistrationDeployment(&deployment)

			Expect(deployment.Policy).To(PointTo(Equal(ondemand)))
		})

		It("should not default the policy field", func() {
			deployment := ControllerRegistrationDeployment{Policy: &always}
			deploymentCopy := deployment.DeepCopy()

			SetDefaults_ControllerRegistrationDeployment(&deployment)

			Expect(deployment.Policy).To(Equal(deploymentCopy.Policy))
		})
	})

	Describe("#SetDefaults_Seed", func() {
		var obj *Seed

		BeforeEach(func() {
			obj = &Seed{}
		})

		It("should default the seed settings (w/o taints)", func() {
			SetDefaults_Seed(obj)

			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeTrue())
			Expect(obj.Spec.Settings.ShootDNS.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(BeTrue())
		})

		It("should allow taints that were not allowed in version v1.12", func() {
			taints := []SeedTaint{
				{Key: "seed.gardener.cloud/disable-capacity-reservation"},
				{Key: "seed.gardener.cloud/disable-dns"},
				{Key: "seed.gardener.cloud/invisible"},
			}
			obj.Spec.Taints = taints

			SetDefaults_Seed(obj)

			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeTrue())
			Expect(obj.Spec.Settings.ShootDNS.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(BeTrue())
			Expect(obj.Spec.Taints).To(HaveLen(3))
			Expect(obj.Spec.Taints).To(Equal(taints))
		})

		It("should not default the seed settings because they were provided", func() {
			var (
				excessCapacityReservation = false
				scheduling                = true
				shootDNS                  = false
				vpaEnabled                = false
			)

			obj.Spec.Settings = &SeedSettings{
				ExcessCapacityReservation: &SeedSettingExcessCapacityReservation{Enabled: excessCapacityReservation},
				Scheduling:                &SeedSettingScheduling{Visible: scheduling},
				ShootDNS:                  &SeedSettingShootDNS{Enabled: shootDNS},
				VerticalPodAutoscaler:     &SeedSettingVerticalPodAutoscaler{Enabled: vpaEnabled},
			}

			SetDefaults_Seed(obj)

			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(Equal(excessCapacityReservation))
			Expect(obj.Spec.Settings.Scheduling.Visible).To(Equal(scheduling))
			Expect(obj.Spec.Settings.ShootDNS.Enabled).To(Equal(shootDNS))
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(Equal(vpaEnabled))
		})
	})

	Describe("#SetDefaults_Shoot", func() {
		var obj *Shoot

		BeforeEach(func() {
			obj = &Shoot{}
		})

		It("should not add the 'protected' toleration if the namespace is not 'garden'", func() {
			obj.Namespace = "foo"
			obj.Spec.Tolerations = nil

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Tolerations).To(BeNil())
		})

		It("should add the 'protected' toleration if the namespace is 'garden'", func() {
			obj.Namespace = "garden"
			obj.Spec.Tolerations = []Toleration{{Key: "foo"}}

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Tolerations).To(ConsistOf(
				Equal(Toleration{Key: "foo"}),
				Equal(Toleration{Key: SeedTaintProtected}),
			))
		})

		It("should default the failSwapOn field", func() {
			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeTrue()))
		})

		It("should not default the failSwapOn field", func() {
			falseVar := false
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
			obj.Spec.Kubernetes.Kubelet.FailSwapOn = &falseVar

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.FailSwapOn).To(PointTo(BeFalse()))
		})

		It("should default the imageGCThreshold fields", func() {
			SetDefaults_Shoot(obj)

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

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent).To(PointTo(Equal(high)))
			Expect(obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent).To(PointTo(Equal(low)))
		})

		It("should not default the kube-controller-manager's pod eviction timeout field", func() {
			podEvictionTimeout := &metav1.Duration{Duration: time.Minute}
			obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{PodEvictionTimeout: podEvictionTimeout}

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout).To(Equal(podEvictionTimeout))
		})

		It("should default the kube-controller-manager's pod eviction timeout field", func() {
			obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{}

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout).To(Equal(&metav1.Duration{Duration: 2 * time.Minute}))
		})

		It("should not default the kube-controller-manager's node monitor grace period", func() {
			nodeMonitorGracePeriod := &metav1.Duration{Duration: time.Minute}
			obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{NodeMonitorGracePeriod: nodeMonitorGracePeriod}

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod).To(Equal(nodeMonitorGracePeriod))
		})

		It("should default the kube-controller-manager's node monitor grace period", func() {
			obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{}

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod).To(Equal(&metav1.Duration{Duration: 2 * time.Minute}))
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
				SetDefaults_Shoot(obj)

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
				SetDefaults_Shoot(obj)

				Expect(obj.Spec.Kubernetes.Kubelet.KubeReserved).To(PointTo(Equal(KubeletConfigReserved{
					CPU:    &kubeReservedCPU,
					Memory: &kubeReservedMemory,
					PID:    &kubeReservedPID,
				})))
			})
		})

		It("should set the maintenance field", func() {
			obj.Spec.Maintenance = nil

			SetDefaults_Shoot(obj)

			Expect(obj.Spec.Maintenance).To(Equal(&Maintenance{}))
		})

		It("should enable basic auth for k8s < 1.16", func() {
			obj.Spec.Kubernetes.Version = "1.15.1"
			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication).To(PointTo(BeTrue()))
		})

		It("should disable basic auth for k8s >= 1.16", func() {
			obj.Spec.Kubernetes.Version = "1.16.1"
			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication).To(PointTo(BeFalse()))
		})

		It("should default the max inflight requests fields", func() {
			SetDefaults_Shoot(obj)
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

			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight).To(Equal(&maxNonMutatingRequestsInflight))
			Expect(obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight).To(Equal(&maxMutatingRequestsInflight))
		})

		It("should default the event ttl field", func() {
			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.EventTTL).To(Equal(&metav1.Duration{Duration: time.Hour}))
		})

		It("should not default the event ttl field", func() {
			eventTTL := &metav1.Duration{Duration: 4 * time.Hour}
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{EventTTL: eventTTL}

			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.EventTTL).To(Equal(eventTTL))
		})

		It("should disable anonymous authentication by default", func() {
			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication).To(PointTo(BeFalse()))
		})

		It("should not default the anonymous authentication field if it is explicitly set", func() {
			trueVar := true
			obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{EnableAnonymousAuthentication: &trueVar}
			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication).To(PointTo(BeTrue()))
		})

		It("should default cri.name for k8s versions >=1.22 to containerd", func() {
			obj.Spec.Kubernetes.Version = "1.22"
			obj.Spec.Provider.Workers = []Worker{
				{Name: "DefaultWorker"},
				{Name: "Worker with CRI configuration",
					CRI: &CRI{Name: CRIName("some configured value")}},
			}
			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Provider.Workers[0].CRI).ToNot(BeNil())
			Expect(obj.Spec.Provider.Workers[0].CRI.Name).To(Equal(CRINameContainerD))
			Expect(obj.Spec.Provider.Workers[1].CRI.Name).To(BeEquivalentTo("some configured value"))
		})

		It("should not default cri.name for k8s versions <1.22", func() {
			obj.Spec.Kubernetes.Version = "1.21"
			obj.Spec.Provider.Workers = []Worker{
				{Name: "DefaultWorker"},
				{Name: "Worker with CRI configuration",
					CRI: &CRI{Name: CRIName("some configured value")}},
			}
			SetDefaults_Shoot(obj)
			Expect(obj.Spec.Provider.Workers[0].CRI).To(BeNil())
			Expect(obj.Spec.Provider.Workers[1].CRI.Name).To(BeEquivalentTo("some configured value"))
		})
	})

	Describe("#SetDefaults_Maintenance", func() {
		var obj *Maintenance

		BeforeEach(func() {
			obj = &Maintenance{}
		})

		It("should correctly default the maintenance", func() {
			obj.AutoUpdate = nil
			obj.TimeWindow = nil

			SetDefaults_Maintenance(obj)

			Expect(obj.AutoUpdate).NotTo(BeNil())
			Expect(obj.AutoUpdate.KubernetesVersion).To(BeTrue())
			Expect(obj.AutoUpdate.MachineImageVersion).To(BeTrue())
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

		It("should not default explicitly set fields", func() {
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
