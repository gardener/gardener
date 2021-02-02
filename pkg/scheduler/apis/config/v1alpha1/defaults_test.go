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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	configv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("SchedulerConfiguration", func() {
		var obj *configv1alpha1.SchedulerConfiguration

		Context("Empty configuration", func() {
			BeforeEach(func() {
				obj = &configv1alpha1.SchedulerConfiguration{}
			})

			It("should correctly default the admission controller configuration", func() {
				configv1alpha1.SetObjectDefaults_SchedulerConfiguration(obj)

				Expect(obj.LogLevel).To(Equal("info"))
				Expect(obj.Server.HTTP.BindAddress).To(Equal("0.0.0.0"))
				Expect(obj.Server.HTTP.Port).To(Equal(10251))
				Expect(obj.Schedulers).To(Equal(configv1alpha1.SchedulerControllerConfiguration{
					BackupBucket: &configv1alpha1.BackupBucketSchedulerConfiguration{
						ConcurrentSyncs: 2,
						RetrySyncPeriod: metav1.Duration{
							Duration: 15 * time.Second,
						},
					},
					Shoot: &configv1alpha1.ShootSchedulerConfiguration{
						ConcurrentSyncs: 5,
						RetrySyncPeriod: metav1.Duration{
							Duration: 15 * time.Second,
						},
						Strategy: configv1alpha1.Default,
					},
				}))
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
	})
})
