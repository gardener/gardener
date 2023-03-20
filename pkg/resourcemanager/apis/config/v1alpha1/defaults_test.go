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

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_ResourceManagerConfiguration", func() {
		It("should default the object", func() {
			obj := &ResourceManagerConfiguration{}

			SetDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("info"))
			Expect(obj.LogFormat).To(Equal("json"))
		})

		It("should not overwrite existing values", func() {
			obj := &ResourceManagerConfiguration{
				LogLevel:  "foo",
				LogFormat: "bar",
			}

			SetDefaults_ResourceManagerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("foo"))
			Expect(obj.LogFormat).To(Equal("bar"))
		})
	})

	Describe("#SetDefaults_SourceClientConnection", func() {
		It("should default the object", func() {
			obj := &SourceClientConnection{}

			SetDefaults_SourceClientConnection(obj)

			Expect(obj.Namespace).To(PointTo(Equal("")))
			Expect(obj.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.QPS).To(Equal(float32(100.0)))
			Expect(obj.Burst).To(Equal(int32(130)))
		})

		It("should not overwrite existing values", func() {
			obj := &SourceClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   float32(1.2),
					Burst: int32(34),
				},
				Namespace:         pointer.String("foo"),
				CacheResyncPeriod: &metav1.Duration{Duration: time.Hour},
			}

			SetDefaults_SourceClientConnection(obj)

			Expect(obj.Namespace).To(PointTo(Equal("foo")))
			Expect(obj.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
			Expect(obj.QPS).To(Equal(float32(1.2)))
			Expect(obj.Burst).To(Equal(int32(34)))
		})
	})

	Describe("#SetDefaults_TargetClientConnection", func() {
		It("should default the object", func() {
			obj := &TargetClientConnection{}

			SetDefaults_TargetClientConnection(obj)

			Expect(obj.Namespace).To(PointTo(Equal("")))
			Expect(obj.DisableCachedClient).To(PointTo(BeFalse()))
			Expect(obj.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))
			Expect(obj.QPS).To(Equal(float32(100.0)))
			Expect(obj.Burst).To(Equal(int32(130)))
		})

		It("should not overwrite existing values", func() {
			obj := &TargetClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   float32(1.2),
					Burst: int32(34),
				},
				Namespace:           pointer.String("foo"),
				DisableCachedClient: pointer.Bool(true),
				CacheResyncPeriod:   &metav1.Duration{Duration: time.Hour},
			}

			SetDefaults_TargetClientConnection(obj)

			Expect(obj.Namespace).To(PointTo(Equal("foo")))
			Expect(obj.DisableCachedClient).To(PointTo(BeTrue()))
			Expect(obj.CacheResyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
			Expect(obj.QPS).To(Equal(float32(1.2)))
			Expect(obj.Burst).To(Equal(int32(34)))
		})
	})

	Describe("#SetDefaults_LeaderElectionConfiguration", func() {
		It("should default the object", func() {
			obj := &componentbaseconfigv1alpha1.LeaderElectionConfiguration{}

			SetDefaults_LeaderElectionConfiguration(obj)

			Expect(obj.ResourceLock).To(Equal("leases"))
			Expect(obj.ResourceName).To(Equal("gardener-resource-manager"))
			Expect(obj.ResourceNamespace).To(BeEmpty())
			Expect(obj.LeaseDuration).To(Equal(metav1.Duration{Duration: 15 * time.Second}))
			Expect(obj.RenewDeadline).To(Equal(metav1.Duration{Duration: 10 * time.Second}))
			Expect(obj.RetryPeriod).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
			Expect(obj.LeaderElect).To(PointTo(BeTrue()))
		})

		It("should not overwrite existing values", func() {
			obj := &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				ResourceLock:      "foo",
				ResourceName:      "bar",
				ResourceNamespace: "baz",
				LeaseDuration:     metav1.Duration{Duration: 1 * time.Second},
				RenewDeadline:     metav1.Duration{Duration: 2 * time.Second},
				RetryPeriod:       metav1.Duration{Duration: 3 * time.Second},
				LeaderElect:       pointer.Bool(false),
			}

			SetDefaults_LeaderElectionConfiguration(obj)

			Expect(obj.ResourceLock).To(Equal("foo"))
			Expect(obj.ResourceName).To(Equal("bar"))
			Expect(obj.ResourceNamespace).To(Equal("baz"))
			Expect(obj.LeaseDuration).To(Equal(metav1.Duration{Duration: 1 * time.Second}))
			Expect(obj.RenewDeadline).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
			Expect(obj.RetryPeriod).To(Equal(metav1.Duration{Duration: 3 * time.Second}))
			Expect(obj.LeaderElect).To(PointTo(BeFalse()))
		})
	})

	Describe("#SetDefaults_ServerConfiguration", func() {
		It("should default the object", func() {
			obj := &ServerConfiguration{}

			SetDefaults_ServerConfiguration(obj)

			Expect(obj.Webhooks.BindAddress).To(Equal("0.0.0.0"))
			Expect(obj.Webhooks.Port).To(Equal(9449))
			Expect(obj.HealthProbes.Port).To(Equal(8081))
			Expect(obj.Metrics.Port).To(Equal(8080))
		})

		It("should not overwrite existing values", func() {
			obj := &ServerConfiguration{
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

			SetDefaults_ServerConfiguration(obj)

			Expect(obj.Webhooks.BindAddress).To(Equal("foo"))
			Expect(obj.Webhooks.Port).To(Equal(1))
			Expect(obj.HealthProbes.Port).To(Equal(2))
			Expect(obj.Metrics.Port).To(Equal(3))
		})
	})

	Describe("#SetDefaults_ResourceManagerControllerConfiguration", func() {
		It("should default the object", func() {
			obj := &ResourceManagerControllerConfiguration{}

			SetDefaults_ResourceManagerControllerConfiguration(obj)

			Expect(obj.ClusterID).To(PointTo(Equal("")))
			Expect(obj.ResourceClass).To(PointTo(Equal("resources")))
		})

		It("should not overwrite existing values", func() {
			obj := &ResourceManagerControllerConfiguration{
				ClusterID:     pointer.String("foo"),
				ResourceClass: pointer.String("bar"),
			}

			SetDefaults_ResourceManagerControllerConfiguration(obj)

			Expect(obj.ClusterID).To(PointTo(Equal("foo")))
			Expect(obj.ResourceClass).To(PointTo(Equal("bar")))
		})
	})

	Describe("#SetDefaults_KubeletCSRApproverControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj := &KubeletCSRApproverControllerConfig{}

			SetDefaults_KubeletCSRApproverControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj := &KubeletCSRApproverControllerConfig{
				Enabled: true,
			}

			SetDefaults_KubeletCSRApproverControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(1)))
		})

		It("should not overwrite existing values", func() {
			obj := &KubeletCSRApproverControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
			}

			SetDefaults_KubeletCSRApproverControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("#SetDefaults_GarbageCollectorControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj := &GarbageCollectorControllerConfig{}

			SetDefaults_GarbageCollectorControllerConfig(obj)

			Expect(obj.SyncPeriod).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj := &GarbageCollectorControllerConfig{
				Enabled: true,
			}

			SetDefaults_GarbageCollectorControllerConfig(obj)

			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
		})

		It("should not overwrite existing values", func() {
			obj := &GarbageCollectorControllerConfig{
				Enabled:    true,
				SyncPeriod: &metav1.Duration{Duration: time.Second},
			}

			SetDefaults_GarbageCollectorControllerConfig(obj)

			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
		})
	})

	Describe("#SetDefaults_HealthControllerConfig", func() {
		It("should not default the object", func() {
			obj := &HealthControllerConfig{}

			SetDefaults_HealthControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})

		It("should not overwrite existing values", func() {
			obj := &HealthControllerConfig{
				ConcurrentSyncs: pointer.Int(1),
				SyncPeriod:      &metav1.Duration{Duration: time.Second},
			}

			SetDefaults_HealthControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(1)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
		})
	})

	Describe("#SetDefaults_ManagedResourceControllerConfig", func() {
		It("should not default the object", func() {
			obj := &ManagedResourceControllerConfig{}

			SetDefaults_ManagedResourceControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			Expect(obj.AlwaysUpdate).To(PointTo(BeFalse()))
			Expect(obj.ManagedByLabelValue).To(PointTo(Equal("gardener")))
		})

		It("should not overwrite existing values", func() {
			obj := &ManagedResourceControllerConfig{
				ConcurrentSyncs:     pointer.Int(1),
				SyncPeriod:          &metav1.Duration{Duration: time.Second},
				AlwaysUpdate:        pointer.Bool(true),
				ManagedByLabelValue: pointer.String("foo"),
			}

			SetDefaults_ManagedResourceControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(1)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
			Expect(obj.AlwaysUpdate).To(PointTo(BeTrue()))
			Expect(obj.ManagedByLabelValue).To(PointTo(Equal("foo")))
		})
	})

	Describe("#SetDefaults_SecretControllerConfig", func() {
		It("should not default the object", func() {
			obj := &SecretControllerConfig{}

			SetDefaults_SecretControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite existing values", func() {
			obj := &SecretControllerConfig{
				ConcurrentSyncs: pointer.Int(1),
			}

			SetDefaults_SecretControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(1)))
		})
	})

	Describe("#SetDefaults_TokenInvalidatorControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj := &TokenInvalidatorControllerConfig{}

			SetDefaults_TokenInvalidatorControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj := &TokenInvalidatorControllerConfig{
				Enabled: true,
			}

			SetDefaults_TokenInvalidatorControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite existing values", func() {
			obj := &TokenInvalidatorControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
			}

			SetDefaults_TokenInvalidatorControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("#SetDefaults_TokenRequestorControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj := &TokenRequestorControllerConfig{}

			SetDefaults_TokenRequestorControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj := &TokenRequestorControllerConfig{
				Enabled: true,
			}

			SetDefaults_TokenRequestorControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite existing values", func() {
			obj := &TokenRequestorControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
			}

			SetDefaults_TokenRequestorControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(2)))
		})
	})

	Describe("#SetDefaults_NodeControllerConfig", func() {
		It("should not default the object because disabled", func() {
			obj := &NodeControllerConfig{}

			SetDefaults_NodeControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj := &NodeControllerConfig{
				Enabled: true,
			}

			SetDefaults_NodeControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Backoff).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Second})))
		})

		It("should not overwrite existing values", func() {
			obj := &NodeControllerConfig{
				Enabled:         true,
				ConcurrentSyncs: pointer.Int(2),
				Backoff:         &metav1.Duration{Duration: time.Minute},
			}

			SetDefaults_NodeControllerConfig(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(2)))
			Expect(obj.Backoff).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
		})
	})

	Describe("#SetDefaults_PodSchedulerNameWebhookConfig", func() {
		It("should not default the object because disabled", func() {
			obj := &PodSchedulerNameWebhookConfig{}

			SetDefaults_PodSchedulerNameWebhookConfig(obj)

			Expect(obj.SchedulerName).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj := &PodSchedulerNameWebhookConfig{
				Enabled: true,
			}

			SetDefaults_PodSchedulerNameWebhookConfig(obj)

			Expect(obj.SchedulerName).To(PointTo(Equal("default-scheduler")))
		})

		It("should not overwrite existing values", func() {
			obj := &PodSchedulerNameWebhookConfig{
				Enabled:       true,
				SchedulerName: pointer.String("foo"),
			}

			SetDefaults_PodSchedulerNameWebhookConfig(obj)

			Expect(obj.SchedulerName).To(PointTo(Equal("foo")))
		})
	})

	Describe("#SetDefaults_ProjectedTokenMountWebhookConfig", func() {
		It("should not default the object because disabled", func() {
			obj := &ProjectedTokenMountWebhookConfig{}

			SetDefaults_ProjectedTokenMountWebhookConfig(obj)

			Expect(obj.ExpirationSeconds).To(BeNil())
		})

		It("should default the object because enabled", func() {
			obj := &ProjectedTokenMountWebhookConfig{
				Enabled: true,
			}

			SetDefaults_ProjectedTokenMountWebhookConfig(obj)

			Expect(obj.ExpirationSeconds).To(PointTo(Equal(int64(43200))))
		})

		It("should not overwrite existing values", func() {
			obj := &ProjectedTokenMountWebhookConfig{
				Enabled:           true,
				ExpirationSeconds: pointer.Int64(1234),
			}

			SetDefaults_ProjectedTokenMountWebhookConfig(obj)

			Expect(obj.ExpirationSeconds).To(PointTo(Equal(int64(1234))))
		})
	})
})
