// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/gardener/gardener/pkg/logger"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	var obj *schedulerconfigv1alpha1.SchedulerConfiguration

	BeforeEach(func() {
		obj = &schedulerconfigv1alpha1.SchedulerConfiguration{}
	})

	Describe("SchedulerConfiguration defaulting", func() {
		It("should default the scheduler configuration", func() {
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatJSON))
			Expect(obj.LeaderElection).NotTo(BeNil())
		})

		It("should not overwrite already set values for scheduler configuration", func() {
			obj = &schedulerconfigv1alpha1.SchedulerConfiguration{
				LogLevel:  schedulerconfigv1alpha1.LogLevelDebug,
				LogFormat: schedulerconfigv1alpha1.LogFormatText,
			}

			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(logger.DebugLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatText))
			Expect(obj.LeaderElection).NotTo(BeNil())
		})
	})

	Describe("SchedulerControllerConfiguration defaulting", func() {
		It("should default the scheduler controller configuration", func() {
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			Expect(obj.Schedulers).To(Equal(schedulerconfigv1alpha1.SchedulerControllerConfiguration{
				BackupBucket: &schedulerconfigv1alpha1.BackupBucketSchedulerConfiguration{
					ConcurrentSyncs: 2,
				},
				Shoot: &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
					ConcurrentSyncs: 5,
					Strategy:        schedulerconfigv1alpha1.Default,
				},
			}))
		})

		It("should not overwrite already set values for scheduler controller configuration", func() {
			obj = &schedulerconfigv1alpha1.SchedulerConfiguration{
				Schedulers: schedulerconfigv1alpha1.SchedulerControllerConfiguration{
					BackupBucket: &schedulerconfigv1alpha1.BackupBucketSchedulerConfiguration{
						ConcurrentSyncs: 3,
					},
					Shoot: &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
						ConcurrentSyncs: 6,
						Strategy:        schedulerconfigv1alpha1.MinimalDistance,
					},
				},
			}

			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			Expect(obj.Schedulers).To(Equal(schedulerconfigv1alpha1.SchedulerControllerConfiguration{
				BackupBucket: &schedulerconfigv1alpha1.BackupBucketSchedulerConfiguration{
					ConcurrentSyncs: 3,
				},
				Shoot: &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
					ConcurrentSyncs: 6,
					Strategy:        schedulerconfigv1alpha1.MinimalDistance,
				},
			}))
		})
	})

	Describe("ServerConfiguration defaulting", func() {
		It("should not overwrite already set values for ServerConfiguration", func() {
			serverConfiguration := &schedulerconfigv1alpha1.ServerConfiguration{
				HealthProbes: &schedulerconfigv1alpha1.Server{
					Port: 1234,
				},
				Metrics: &schedulerconfigv1alpha1.Server{
					Port: 1235,
				},
			}
			obj.Server = *serverConfiguration

			expectedServerConfiguration := serverConfiguration.DeepCopy()

			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)
			Expect(obj.Server).To(Equal(*expectedServerConfiguration))
		})

		It("should default values for ServerConfiguration", func() {
			obj.Server = schedulerconfigv1alpha1.ServerConfiguration{}

			expectedServerConfiguration := schedulerconfigv1alpha1.ServerConfiguration{
				HealthProbes: &schedulerconfigv1alpha1.Server{
					Port: 10251,
				},
				Metrics: &schedulerconfigv1alpha1.Server{
					Port: 19251,
				},
			}

			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)
			Expect(obj.Server).To(Equal(expectedServerConfiguration))
		})
	})

	Describe("ClientConnection defaulting", func() {
		It("should not default ContentType and AcceptContentTypes", func() {
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
			// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the intelligent
			// logic will be overwritten
			Expect(obj.ClientConnection.ContentType).To(BeEmpty())
			Expect(obj.ClientConnection.AcceptContentTypes).To(BeEmpty())
		})

		It("should correctly default ClientConnection", func() {
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)
			Expect(obj.ClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   50.0,
				Burst: 100,
			}))
		})

		It("should not overwrite already set values for ClientConnection", func() {
			obj.ClientConnection = componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   60.0,
				Burst: 110,
			}

			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			Expect(obj.ClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   60.0,
				Burst: 110,
			}))
		})
	})

	Describe("LeaderElection defaulting", func() {
		It("should default leader election settings", func() {
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			Expect(obj.LeaderElection).NotTo(BeNil())
			Expect(obj.LeaderElection.LeaderElect).To(PointTo(BeTrue()))
			Expect(obj.LeaderElection.LeaseDuration).To(Equal(metav1.Duration{Duration: 15 * time.Second}))
			Expect(obj.LeaderElection.RenewDeadline).To(Equal(metav1.Duration{Duration: 10 * time.Second}))
			Expect(obj.LeaderElection.RetryPeriod).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
			Expect(obj.LeaderElection.ResourceLock).To(Equal("leases"))
			Expect(obj.LeaderElection.ResourceNamespace).To(Equal("garden"))
			Expect(obj.LeaderElection.ResourceName).To(Equal("gardener-scheduler-leader-election"))
		})

		It("should not overwrite already set values for leader election", func() {
			expectedLeaderElection := &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				LeaderElect:       ptr.To(true),
				ResourceLock:      "foo",
				RetryPeriod:       metav1.Duration{Duration: 40 * time.Second},
				RenewDeadline:     metav1.Duration{Duration: 41 * time.Second},
				LeaseDuration:     metav1.Duration{Duration: 42 * time.Second},
				ResourceNamespace: "other-garden-ns",
				ResourceName:      "lock-object",
			}
			obj.LeaderElection = expectedLeaderElection.DeepCopy()
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

			Expect(obj.LeaderElection).To(Equal(expectedLeaderElection))
		})
	})
})

var _ = Describe("Constants", func() {
	It("should have the same values as the corresponding constants in the logger package", func() {
		Expect(schedulerconfigv1alpha1.LogLevelDebug).To(Equal(logger.DebugLevel))
		Expect(schedulerconfigv1alpha1.LogLevelInfo).To(Equal(logger.InfoLevel))
		Expect(schedulerconfigv1alpha1.LogLevelError).To(Equal(logger.ErrorLevel))
		Expect(schedulerconfigv1alpha1.LogFormatJSON).To(Equal(logger.FormatJSON))
		Expect(schedulerconfigv1alpha1.LogFormatText).To(Equal(logger.FormatText))
	})
})
