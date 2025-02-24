// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
)

var _ = Describe("ResourceManager defaulting", func() {
	var obj *ResourceManagerConfiguration

	BeforeEach(func() {
		obj = &ResourceManagerConfiguration{}
	})

	Describe("ResourceManagerConfiguration defaulting", func() {
		It("should default the ResourceManagerConfiguration", func() {
			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("info"))
			Expect(obj.LogFormat).To(Equal("json"))
		})

		It("should not overwrite already set values for ResourceManagerConfiguration", func() {
			obj = &ResourceManagerConfiguration{
				LogLevel:  "foo",
				LogFormat: "bar",
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("foo"))
			Expect(obj.LogFormat).To(Equal("bar"))
		})
	})

	Describe("SourceClientConnection defaulting", func() {
		It("should default the ClientConnection", func() {
			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.SourceClientConnection.Namespaces).To(BeEmpty())
			Expect(obj.SourceClientConnection.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.SourceClientConnection.QPS).To(Equal(float32(100.0)))
			Expect(obj.SourceClientConnection.Burst).To(Equal(int32(130)))
		})

		It("should not overwrite already set values for ClientConnection", func() {
			obj.SourceClientConnection = ClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   float32(1.2),
					Burst: int32(34),
				},
				Namespaces:        []string{"foo"},
				CacheResyncPeriod: &metav1.Duration{Duration: time.Hour},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.SourceClientConnection.Namespaces).To(ConsistOf("foo"))
			Expect(obj.SourceClientConnection.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
			Expect(obj.SourceClientConnection.QPS).To(Equal(float32(1.2)))
			Expect(obj.SourceClientConnection.Burst).To(Equal(int32(34)))
		})
	})

	Describe("TargetClientConnection defaulting", func() {
		It("should default the ClientConnection", func() {
			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.TargetClientConnection.Namespaces).To(BeEmpty())
			Expect(obj.TargetClientConnection.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.TargetClientConnection.QPS).To(Equal(float32(100.0)))
			Expect(obj.TargetClientConnection.Burst).To(Equal(int32(130)))
		})

		It("should not overwrite already set values for ClientConnection", func() {
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

	Describe("LeaderElectionConfiguration defaulting", func() {
		It("should default the LeaderElectionConfiguration", func() {
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

		It("should not overwrite already set values for LeaderElectionConfiguration", func() {
			obj.LeaderElection = componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				ResourceLock:      "foo",
				ResourceName:      "bar",
				ResourceNamespace: "baz",
				LeaseDuration:     metav1.Duration{Duration: 1 * time.Second},
				RenewDeadline:     metav1.Duration{Duration: 2 * time.Second},
				RetryPeriod:       metav1.Duration{Duration: 3 * time.Second},
				LeaderElect:       ptr.To(false),
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

	Describe("ServerConfiguration defaulting", func() {
		It("should default the ServerConfiguration", func() {
			obj.Server = ServerConfiguration{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Server.Webhooks.BindAddress).To(BeEmpty())
			Expect(obj.Server.Webhooks.Port).To(Equal(9449))
			Expect(obj.Server.HealthProbes.Port).To(Equal(8081))
			Expect(obj.Server.Metrics.Port).To(Equal(8080))
		})

		It("should not overwrite already set values for ServerConfiguration", func() {
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

	Describe("ResourceManagerControllerConfiguration defaulting", func() {
		It("should default the ResourceManagerControllerConfiguration", func() {
			obj.Controllers = ResourceManagerControllerConfiguration{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ClusterID).To(PointTo(Equal("")))
			Expect(obj.Controllers.ResourceClass).To(PointTo(Equal("resources")))
		})

		It("should not overwrite already set values for ResourceManagerControllerConfiguration", func() {
			obj.Controllers = ResourceManagerControllerConfiguration{
				ClusterID:     ptr.To("foo"),
				ResourceClass: ptr.To("bar"),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ClusterID).To(PointTo(Equal("foo")))
			Expect(obj.Controllers.ResourceClass).To(PointTo(Equal("bar")))
		})
	})

	Describe("CSRApproverControllerConfig defaulting", func() {
		It("should not default the CSRApproverControllerConfig because it is disabled", func() {
			obj.Controllers.CSRApprover = CSRApproverControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.CSRApprover.ConcurrentSyncs).To(BeNil())
		})

		It("should default the CSRApproverControllerConfig because it is enabled", func() {
			obj.Controllers.CSRApprover = CSRApproverControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.CSRApprover.ConcurrentSyncs).To(PointTo(Equal(1)))
		})

		It("should not overwrite already set values for CSRApproverControllerConfig", func() {
			obj.Controllers.CSRApprover = CSRApproverControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: ptr.To(2),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.CSRApprover.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("GarbageCollectorControllerConfig defaulting", func() {
		It("should not default the GarbageCollectorControllerConfig because it is disabled", func() {
			obj.Controllers.GarbageCollector = GarbageCollectorControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.GarbageCollector.SyncPeriod).To(BeNil())
		})

		It("should default the GarbageCollectorControllerConfig because it is enabled", func() {
			obj.Controllers.GarbageCollector = GarbageCollectorControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.GarbageCollector.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
		})

		It("should not overwrite already set values for GarbageCollectorControllerConfig", func() {
			obj.Controllers.GarbageCollector = GarbageCollectorControllerConfig{
				Enabled:    true,
				SyncPeriod: &metav1.Duration{Duration: time.Second},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.GarbageCollector.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
		})
	})

	Describe("NetworkPolicyConfig defaulting", func() {
		It("should not default the NetworkPolicyConfig because it is disabled", func() {
			obj.Controllers.NetworkPolicy = NetworkPolicyControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(BeNil())
		})

		It("should default the NetworkPolicyConfig because it is enabled", func() {
			obj.Controllers.NetworkPolicy = NetworkPolicyControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite already set values for NetworkPolicyConfig", func() {
			obj.Controllers.NetworkPolicy = NetworkPolicyControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: ptr.To(6),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(PointTo(Equal(6)))
		})
	})

	Describe("HealthControllerConfig defaulting", func() {
		It("should default the HealthControllerConfig", func() {
			obj.Controllers.Health = HealthControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Health.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.Health.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})

		It("should not overwrite already set values for HealthControllerConfig", func() {
			obj.Controllers.Health = HealthControllerConfig{
				ConcurrentSyncs: ptr.To(1),
				SyncPeriod:      &metav1.Duration{Duration: time.Second},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.Health.ConcurrentSyncs).To(PointTo(Equal(1)))
			Expect(obj.Controllers.Health.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
		})
	})

	Describe("ManagedResourceControllerConfig defaulting", func() {
		It("should default the ManagedResourceControllerConfig", func() {
			obj.Controllers.ManagedResource = ManagedResourceControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ManagedResource.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.ManagedResource.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			Expect(obj.Controllers.ManagedResource.AlwaysUpdate).To(PointTo(BeFalse()))
			Expect(obj.Controllers.ManagedResource.ManagedByLabelValue).To(PointTo(Equal("gardener")))
		})

		It("should not overwrite already set values for ManagedResourceControllerConfig", func() {
			obj.Controllers.ManagedResource = ManagedResourceControllerConfig{
				ConcurrentSyncs:     ptr.To(1),
				SyncPeriod:          &metav1.Duration{Duration: time.Second},
				AlwaysUpdate:        ptr.To(true),
				ManagedByLabelValue: ptr.To("foo"),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.ManagedResource.ConcurrentSyncs).To(PointTo(Equal(1)))
			Expect(obj.Controllers.ManagedResource.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
			Expect(obj.Controllers.ManagedResource.AlwaysUpdate).To(PointTo(BeTrue()))
			Expect(obj.Controllers.ManagedResource.ManagedByLabelValue).To(PointTo(Equal("foo")))
		})
	})

	Describe("TokenRequestorControllerConfig defaulting", func() {
		It("should not default the TokenRequestorControllerConfig because it is disabled", func() {
			obj.Controllers.TokenRequestor = TokenRequestorControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenRequestor.ConcurrentSyncs).To(BeNil())
		})

		It("should default the TokenRequestorControllerConfig because it is enabled", func() {
			obj.Controllers.TokenRequestor = TokenRequestorControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenRequestor.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite already set values for TokenRequestorControllerConfig", func() {
			obj.Controllers.TokenRequestor = TokenRequestorControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: ptr.To(2),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.TokenRequestor.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("NodeCriticalComponentsControllerConfig defaulting", func() {
		It("should not default the NodeCriticalComponentsControllerConfig because it is disabled", func() {
			obj.Controllers.NodeCriticalComponents = NodeCriticalComponentsControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NodeCriticalComponents.ConcurrentSyncs).To(BeNil())
		})

		It("should default the NodeCriticalComponentsControllerConfig because it is enabled", func() {
			obj.Controllers.NodeCriticalComponents = NodeCriticalComponentsControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NodeCriticalComponents.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.NodeCriticalComponents.Backoff).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Second})))
		})

		It("should not overwrite already set values for NodeCriticalComponentsControllerConfig", func() {
			obj.Controllers.NodeCriticalComponents = NodeCriticalComponentsControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: ptr.To(2),
				Backoff:         &metav1.Duration{Duration: time.Minute},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NodeCriticalComponents.ConcurrentSyncs).To(PointTo(Equal(2)))
			Expect(obj.Controllers.NodeCriticalComponents.Backoff).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})
	})

	Describe("NodeAgentReconciliationDelayControllerConfig defaulting", func() {
		It("should not default the NodeAgentReconciliationDelayControllerConfig because it is disabled", func() {
			obj.Controllers.NodeAgentReconciliationDelay = NodeAgentReconciliationDelayControllerConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NodeAgentReconciliationDelay.MinDelay).To(BeNil())
			Expect(obj.Controllers.NodeAgentReconciliationDelay.MaxDelay).To(BeNil())
		})

		It("should default the NodeAgentReconciliationDelayControllerConfig because it is enabled", func() {
			obj.Controllers.NodeAgentReconciliationDelay = NodeAgentReconciliationDelayControllerConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NodeAgentReconciliationDelay.MinDelay).To(PointTo(Equal(metav1.Duration{})))
			Expect(obj.Controllers.NodeAgentReconciliationDelay.MaxDelay).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
		})

		It("should not overwrite already set values for NodeAgentReconciliationDelayControllerConfig", func() {
			obj.Controllers.NodeAgentReconciliationDelay = NodeAgentReconciliationDelayControllerConfig{
				Enabled:  true,
				MinDelay: &metav1.Duration{Duration: time.Minute},
				MaxDelay: &metav1.Duration{Duration: time.Hour},
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Controllers.NodeAgentReconciliationDelay.MinDelay).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			Expect(obj.Controllers.NodeAgentReconciliationDelay.MaxDelay).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
		})
	})

	Describe("PodSchedulerNameWebhookConfig defaulting", func() {
		It("should not default the PodSchedulerNameWebhookConfig because it is disabled", func() {
			obj.Webhooks.PodSchedulerName = PodSchedulerNameWebhookConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.PodSchedulerName.SchedulerName).To(BeNil())
		})

		It("should default the PodSchedulerNameWebhookConfig because it is enabled", func() {
			obj.Webhooks.PodSchedulerName = PodSchedulerNameWebhookConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.PodSchedulerName.SchedulerName).To(PointTo(Equal("default-scheduler")))
		})

		It("should not overwrite already set values for PodSchedulerNameWebhookConfig", func() {
			obj.Webhooks.PodSchedulerName = PodSchedulerNameWebhookConfig{
				Enabled:       true,
				SchedulerName: ptr.To("foo"),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.PodSchedulerName.SchedulerName).To(PointTo(Equal("foo")))
		})
	})

	Describe("ProjectedTokenMountWebhookConfig defaulting", func() {
		It("should not default the ProjectedTokenMountWebhookConfig because it is disabled", func() {
			obj.Webhooks.ProjectedTokenMount = ProjectedTokenMountWebhookConfig{}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.ProjectedTokenMount.ExpirationSeconds).To(BeNil())
		})

		It("should default the ProjectedTokenMountWebhookConfig because it is enabled", func() {
			obj.Webhooks.ProjectedTokenMount = ProjectedTokenMountWebhookConfig{
				Enabled: true,
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.ProjectedTokenMount.ExpirationSeconds).To(PointTo(Equal(int64(43200))))
		})

		It("should not overwrite already set values for ProjectedTokenMountWebhookConfig", func() {
			obj.Webhooks.ProjectedTokenMount = ProjectedTokenMountWebhookConfig{
				Enabled:           true,
				ExpirationSeconds: ptr.To[int64](1234),
			}

			SetObjectDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.Webhooks.ProjectedTokenMount.ExpirationSeconds).To(PointTo(Equal(int64(1234))))
		})
	})
})
