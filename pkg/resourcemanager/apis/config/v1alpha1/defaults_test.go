// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
)

var _ = Describe("ResourceManager defaulting", func() {
	var obj *ResourceManagerConfiguration

	BeforeEach(func() {
		obj = &ResourceManagerConfiguration{}
	})

	Describe("#SetDefaults_ResourceManagerConfiguration", func() {
		It("should default the object", func() {
			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("info"))
			Expect(obj.LogFormat).To(Equal("json"))
		})

		It("should not overwrite existing values", func() {
			obj = &ResourceManagerConfiguration{
				LogLevel:  "foo",
				LogFormat: "bar",
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("foo"))
			Expect(obj.LogFormat).To(Equal("bar"))
		})
	})

	Describe("#SetDefaults_ClientConnection", func() {
		It("should default the object", func() {
			obj.TargetClientConnection = &ClientConnection{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.TargetClientConnection.Namespaces).To(BeEmpty())
			Expect(obj.TargetClientConnection.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.TargetClientConnection.QPS).To(Equal(float32(100.0)))
			Expect(obj.TargetClientConnection.Burst).To(Equal(int32(130)))
		})

		It("should not overwrite existing values", func() {
			obj.TargetClientConnection = &ClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   float32(1.2),
					Burst: int32(34),
				},
				Namespaces:        []string{"foo"},
				CacheResyncPeriod: &metav1.Duration{Duration: time.Hour},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.TargetClientConnection.Namespaces).To(ConsistOf("foo"))
			Expect(obj.TargetClientConnection.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
			Expect(obj.TargetClientConnection.QPS).To(Equal(float32(1.2)))
			Expect(obj.TargetClientConnection.Burst).To(Equal(int32(34)))
		})
	})

	Describe("#SetDefaults_LeaderElectionConfiguration", func() {
		It("should default the object", func() {
			obj.LeaderElection = componentbaseconfigv1alpha1.LeaderElectionConfiguration{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LeaderElection.ResourceLock).To(Equal("leases"))
			Expect(obj.LeaderElection.ResourceName).To(Equal("gardener-resource-manager"))
			Expect(obj.LeaderElection.ResourceNamespace).To(BeEmpty())
			Expect(obj.LeaderElection.LeaseDuration).To(Equal(metav1.Duration{Duration: 15 * time.Second}))
			Expect(obj.LeaderElection.RenewDeadline).To(Equal(metav1.Duration{Duration: 10 * time.Second}))
			Expect(obj.LeaderElection.RetryPeriod).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
			Expect(obj.LeaderElection.LeaderElect).To(PointTo(BeTrue()))
		})

		It("should not overwrite existing values", func() {
			obj.LeaderElection = componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				ResourceLock:      "foo",
				ResourceName:      "bar",
				ResourceNamespace: "baz",
				LeaseDuration:     metav1.Duration{Duration: 1 * time.Second},
				RenewDeadline:     metav1.Duration{Duration: 2 * time.Second},
				RetryPeriod:       metav1.Duration{Duration: 3 * time.Second},
				LeaderElect:       pointer.Bool(false),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LeaderElection.ResourceLock).To(Equal("foo"))
			Expect(obj.LeaderElection.ResourceName).To(Equal("bar"))
			Expect(obj.LeaderElection.ResourceNamespace).To(Equal("baz"))
			Expect(obj.LeaderElection.LeaseDuration).To(Equal(metav1.Duration{Duration: 1 * time.Second}))
			Expect(obj.LeaderElection.RenewDeadline).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
			Expect(obj.LeaderElection.RetryPeriod).To(Equal(metav1.Duration{Duration: 3 * time.Second}))
			Expect(obj.LeaderElection.LeaderElect).To(PointTo(BeFalse()))
		})
	})

	Describe("#SetDefaults_ServerConfiguration", func() {
		It("should default the object", func() {
			obj.Server = ServerConfiguration{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Server.Webhooks.BindAddress).To(BeEmpty())
			Expect(obj.Server.Webhooks.Port).To(Equal(9449))
			Expect(obj.Server.HealthProbes.Port).To(Equal(8081))
			Expect(obj.Server.Metrics.Port).To(Equal(8080))
		})

		It("should not overwrite existing values", func() {
			obj.Server = ServerConfiguration{
				Webhooks: HTTPSServer{
					Server: Server{
						BindAddress: "foo",
						Port:        1,
					},
				},
				HealthProbes: &Server{
					Port: 2,
				},
				Metrics: &Server{
					Port: 3,
				},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Server.Webhooks.BindAddress).To(Equal("foo"))
			Expect(obj.Server.Webhooks.Port).To(Equal(1))
			Expect(obj.Server.HealthProbes.Port).To(Equal(2))
			Expect(obj.Server.Metrics.Port).To(Equal(3))
		})
	})

	Describe("#SetDefaults_ResourceManagerControllerConfiguration", func() {
		It("should default the object", func() {
			obj.Controllers = ResourceManagerControllerConfiguration{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ClusterID).To(PointTo(Equal("")))
			Expect(obj.Controllers.ResourceClass).To(PointTo(Equal("resources")))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers = ResourceManagerControllerConfiguration{
				ClusterID:     pointer.String("foo"),
				ResourceClass: pointer.String("bar"),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ClusterID).To(PointTo(Equal("foo")))
			Expect(obj.Controllers.ResourceClass).To(PointTo(Equal("bar")))
		})
	})

	Describe("#SetDefaults_KubeletCSRApproverControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj.Controllers.KubeletCSRApprover = KubeletCSRApproverControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.KubeletCSRApprover.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj.Controllers.KubeletCSRApprover = KubeletCSRApproverControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.KubeletCSRApprover.ConcurrentSyncs).To(PointTo(Equal(1)))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.KubeletCSRApprover = KubeletCSRApproverControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.KubeletCSRApprover.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("#SetDefaults_GarbageCollectorControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj.Controllers.GarbageCollector = GarbageCollectorControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.GarbageCollector.SyncPeriod).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj.Controllers.GarbageCollector = GarbageCollectorControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.GarbageCollector.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.GarbageCollector = GarbageCollectorControllerConfig{
				Enabled:    true,
				SyncPeriod: &metav1.Duration{Duration: time.Second},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.GarbageCollector.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
		})
	})

	Describe("#SetDefaults_NetworkPolicyConfig", func() {
		It("should not default the object", func() {
			obj.Controllers.NetworkPolicy = NetworkPolicyControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object", func() {
			obj.Controllers.NetworkPolicy = NetworkPolicyControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.NetworkPolicy = NetworkPolicyControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(6),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(PointTo(Equal(6)))
		})
	})

	Describe("#SetDefaults_HealthControllerConfig", func() {
		It("should not default the object", func() {
			obj.Controllers.Health = HealthControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Health.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.Health.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.Health = HealthControllerConfig{
				ConcurrentSyncs: pointer.Int(1),
				SyncPeriod:      &metav1.Duration{Duration: time.Second},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Health.ConcurrentSyncs).To(PointTo(Equal(1)))
			Expect(obj.Controllers.Health.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
		})
	})

	Describe("#SetDefaults_ManagedResourceControllerConfig", func() {
		It("should not default the object", func() {
			obj.Controllers.ManagedResource = ManagedResourceControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ManagedResource.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.ManagedResource.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			Expect(obj.Controllers.ManagedResource.AlwaysUpdate).To(PointTo(BeFalse()))
			Expect(obj.Controllers.ManagedResource.ManagedByLabelValue).To(PointTo(Equal("gardener")))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.ManagedResource = ManagedResourceControllerConfig{
				ConcurrentSyncs:     pointer.Int(1),
				SyncPeriod:          &metav1.Duration{Duration: time.Second},
				AlwaysUpdate:        pointer.Bool(true),
				ManagedByLabelValue: pointer.String("foo"),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ManagedResource.ConcurrentSyncs).To(PointTo(Equal(1)))
			Expect(obj.Controllers.ManagedResource.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
			Expect(obj.Controllers.ManagedResource.AlwaysUpdate).To(PointTo(BeTrue()))
			Expect(obj.Controllers.ManagedResource.ManagedByLabelValue).To(PointTo(Equal("foo")))
		})
	})

	Describe("#SetDefaults_SecretControllerConfig", func() {
		It("should not default the object", func() {
			obj.Controllers.Secret = SecretControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Secret.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.Secret = SecretControllerConfig{
				ConcurrentSyncs: pointer.Int(1),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Secret.ConcurrentSyncs).To(PointTo(Equal(1)))
		})
	})

	Describe("#SetDefaults_TokenInvalidatorControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj.Controllers.TokenInvalidator = TokenInvalidatorControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenInvalidator.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj.Controllers.TokenInvalidator = TokenInvalidatorControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenInvalidator.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.TokenInvalidator = TokenInvalidatorControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenInvalidator.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("#SetDefaults_TokenRequestorControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj.Controllers.TokenRequestor = TokenRequestorControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenRequestor.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj.Controllers.TokenRequestor = TokenRequestorControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenRequestor.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.TokenRequestor = TokenRequestorControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenRequestor.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("#SetDefaults_NodeControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj.Controllers.Node = NodeControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Node.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj.Controllers.Node = NodeControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Node.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.Node.Backoff).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Second})))
		})

		It("should not overwrite existing values", func() {
			obj.Controllers.Node = NodeControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
				Backoff:         &metav1.Duration{Duration: time.Minute},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Node.ConcurrentSyncs).To(PointTo(Equal(2)))
			Expect(obj.Controllers.Node.Backoff).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})
	})

	Describe("#SetDefaults_PodSchedulerNameWebhookConfig", func() {
		It("should not default the object because disabled", func() {
			obj.Webhooks.PodSchedulerName = PodSchedulerNameWebhookConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.PodSchedulerName.SchedulerName).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj.Webhooks.PodSchedulerName = PodSchedulerNameWebhookConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.PodSchedulerName.SchedulerName).To(PointTo(Equal("default-scheduler")))
		})

		It("should not overwrite existing values", func() {
			obj.Webhooks.PodSchedulerName = PodSchedulerNameWebhookConfig{
				Enabled:       true,
				SchedulerName: pointer.String("foo"),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.PodSchedulerName.SchedulerName).To(PointTo(Equal("foo")))
		})
	})

	Describe("#SetDefaults_ProjectedTokenMountWebhookConfig", func() {
		It("should not default the object because disabled", func() {
			obj.Webhooks.ProjectedTokenMount = ProjectedTokenMountWebhookConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.ProjectedTokenMount.ExpirationSeconds).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj.Webhooks.ProjectedTokenMount = ProjectedTokenMountWebhookConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.ProjectedTokenMount.ExpirationSeconds).To(PointTo(Equal(int64(43200))))
		})

		It("should not overwrite existing values", func() {
			obj.Webhooks.ProjectedTokenMount = ProjectedTokenMountWebhookConfig{
				Enabled:           true,
				ExpirationSeconds: pointer.Int64(1234),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.ProjectedTokenMount.ExpirationSeconds).To(PointTo(Equal(int64(1234))))
		})
	})
})
