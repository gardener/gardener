// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/logger"
	configv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("SchedulerConfiguration", func() {
		var obj *configv1alpha1.SchedulerConfiguration

		BeforeEach(func() {
			obj = &configv1alpha1.SchedulerConfiguration{}
		})

		Context("Empty configuration", func() {
			It("should correctly default the admission controller configuration", func() {
				configv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

				Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
				Expect(obj.LogFormat).To(Equal(logger.FormatJSON))
				Expect(obj.Schedulers).To(Equal(configv1alpha1.SchedulerControllerConfiguration{
					BackupBucket: &configv1alpha1.BackupBucketSchedulerConfiguration{
						ConcurrentSyncs: 2,
					},
					Shoot: &configv1alpha1.ShootSchedulerConfiguration{
						ConcurrentSyncs: 5,
						Strategy:        configv1alpha1.Default,
					},
				}))
			})
		})

		Describe("ServerConfiguration", func() {
			It("should not default any values for ServerConfiguration", func() {
				serverConfiguration := &configv1alpha1.ServerConfiguration{
					HealthProbes: &configv1alpha1.Server{
						BindAddress: "127.0.0.1",
						Port:        1234,
					},
					Metrics: &configv1alpha1.Server{
						BindAddress: "10.0.0.1",
						Port:        1235,
					},
				}

				expectedServerConfiguration := serverConfiguration.DeepCopy()

				configv1alpha1.SetDefaults_ServerConfiguration(serverConfiguration)
				Expect(serverConfiguration).To(Equal(expectedServerConfiguration))
			})

			It("should default values for ServerConfiguration", func() {
				serverConfiguration := &configv1alpha1.ServerConfiguration{}

				expectedServerConfiguration := &configv1alpha1.ServerConfiguration{
					HealthProbes: &configv1alpha1.Server{
						BindAddress: "0.0.0.0",
						Port:        10251,
					},
					Metrics: &configv1alpha1.Server{
						BindAddress: "0.0.0.0",
						Port:        19251,
					},
				}

				configv1alpha1.SetDefaults_ServerConfiguration(serverConfiguration)
				Expect(serverConfiguration).To(Equal(expectedServerConfiguration))
			})
		})

		Describe("ClientConnection", func() {
			It("should not default ContentType and AcceptContentTypes", func() {
				configv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

				// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
				// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the integelligent
				// logic will be overwritten
				Expect(obj.ClientConnection.ContentType).To(BeEmpty())
				Expect(obj.ClientConnection.AcceptContentTypes).To(BeEmpty())
			})
			It("should correctly default ClientConnection", func() {
				configv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)
				Expect(obj.ClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   50.0,
					Burst: 100,
				}))
			})
		})

		Describe("leader election settings", func() {
			It("should correctly default leader election settings", func() {
				configv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

				Expect(obj.LeaderElection).NotTo(BeNil())
				Expect(obj.LeaderElection.LeaderElect).To(PointTo(BeTrue()))
				Expect(obj.LeaderElection.LeaseDuration).To(Equal(metav1.Duration{Duration: 15 * time.Second}))
				Expect(obj.LeaderElection.RenewDeadline).To(Equal(metav1.Duration{Duration: 10 * time.Second}))
				Expect(obj.LeaderElection.RetryPeriod).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
				Expect(obj.LeaderElection.ResourceLock).To(Equal("leases"))
				Expect(obj.LeaderElection.ResourceNamespace).To(Equal("garden"))
				Expect(obj.LeaderElection.ResourceName).To(Equal("gardener-scheduler-leader-election"))
			})
			It("should not overwrite custom settings", func() {
				expectedLeaderElection := &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       pointer.Bool(true),
					ResourceLock:      "foo",
					RetryPeriod:       metav1.Duration{Duration: 40 * time.Second},
					RenewDeadline:     metav1.Duration{Duration: 41 * time.Second},
					LeaseDuration:     metav1.Duration{Duration: 42 * time.Second},
					ResourceNamespace: "other-garden-ns",
					ResourceName:      "lock-object",
				}
				obj.LeaderElection = *expectedLeaderElection.DeepCopy()
				configv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

				Expect(obj.LeaderElection).To(Equal(*expectedLeaderElection))
			})
		})
	})
})
