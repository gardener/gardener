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

package validation_test

import (
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	. "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("ValidateScheduler", func() {
	var (
		schedulerConfiguration imports.GardenerScheduler
		componentConfig        schedulerconfigv1alpha1.SchedulerConfiguration
		path                   = field.NewPath("scheduler")
	)

	BeforeEach(func() {
		componentConfig = schedulerconfigv1alpha1.SchedulerConfiguration{
			Schedulers: schedulerconfigv1alpha1.SchedulerControllerConfiguration{
				BackupBucket: nil,
				Shoot: &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
					Strategy: schedulerconfigv1alpha1.MinimalDistance,
				},
			},
		}

		schedulerConfiguration = imports.GardenerScheduler{
			DeploymentConfiguration: &imports.CommonDeploymentConfiguration{
				ReplicaCount:       pointer.Int32(1),
				ServiceAccountName: pointer.String("sx"),
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu": resource.MustParse("2"),
					},
					Requests: corev1.ResourceList{
						"memory": resource.MustParse("3Gi"),
					},
				},
				PodLabels:      map[string]string{"foo": "bar"},
				PodAnnotations: map[string]string{"foo": "annotation"},
				VPA:            pointer.Bool(true),
			},
			ComponentConfiguration: &imports.SchedulerComponentConfiguration{
				Configuration: &imports.Configuration{
					ComponentConfiguration: &componentConfig,
				},
			},
		}
	})

	Describe("#Validate Component Configuration", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateScheduler(schedulerConfiguration, path)
			Expect(errorList).To(BeEmpty())
		})

		// Demonstrate that the Schedulers component configuration is validated.
		// Otherwise, the component might fail after deployment by the landscaper
		It("should forbid invalid configurations", func() {
			componentConfig.Schedulers.Shoot = &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
				Strategy: "not-existing-strategy",
			}
			errorList := ValidateScheduler(schedulerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("scheduler.componentConfiguration.config.schedulers.shoot.strategy"),
				})),
			))
		})
	})

	Context("validate the Scheduler's deployment configuration", func() {
		It("should validate that the replica count is not negative", func() {
			schedulerConfiguration.DeploymentConfiguration.ReplicaCount = pointer.Int32(-1)

			errorList := ValidateScheduler(schedulerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("scheduler.deploymentConfiguration.replicaCount"),
				})),
			))
		})

		It("should validate that the service account name is valid", func() {
			schedulerConfiguration.DeploymentConfiguration.ServiceAccountName = pointer.String("x121ä232..")

			errorList := ValidateScheduler(schedulerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("scheduler.deploymentConfiguration.serviceAccountName"),
				})),
			))
		})

		It("should validate that the pod labels are valid", func() {
			schedulerConfiguration.DeploymentConfiguration.PodLabels = map[string]string{"foo!": "bar"}

			errorList := ValidateScheduler(schedulerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("scheduler.deploymentConfiguration.podLabels"),
				})),
			))
		})

		It("should validate that the podAnnotations are valid", func() {
			schedulerConfiguration.DeploymentConfiguration.PodAnnotations = map[string]string{"bar@": "baz"}

			errorList := ValidateScheduler(schedulerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("scheduler.deploymentConfiguration.podAnnotations"),
				})),
			))
		})
	})
})
