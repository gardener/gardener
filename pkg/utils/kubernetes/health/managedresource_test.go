// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Managedresource", func() {
	Context("#CheckManagedResource", func() {
		DescribeTable("managedresource",
			func(mr resourcesv1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := health.CheckManagedResource(&mr)
				Expect(err).To(matcher)
			},
			Entry("applied condition not true", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionFalse,
						},
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("healthy condition not true", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
						},
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("conditions true", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
						},
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}, Not(HaveOccurred())),
			Entry("no applied condition", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("no healthy condition", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("no conditions", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("outdated generation", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("no status", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
			}, HaveOccurred()),
		)
	})

	Context("#CheckManagedResourceApplied", func() {
		DescribeTable("managedresource",
			func(mr resourcesv1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := health.CheckManagedResourceApplied(&mr)
				Expect(err).To(matcher)
			},
			Entry("applied condition not true", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("condition true", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}, Not(HaveOccurred())),
			Entry("no applied condition", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions:         []gardencorev1beta1.Condition{},
				},
			}, HaveOccurred()),
			Entry("no conditions", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("outdated generation", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("no status", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
			}, HaveOccurred()),
		)
	})

	Context("#CheckManagedResourceHealthy", func() {
		DescribeTable("managedresource",
			func(mr resourcesv1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := health.CheckManagedResourceHealthy(&mr)
				Expect(err).To(matcher)
			},
			Entry("healthy condition not true", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("condition true", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}, Not(HaveOccurred())),
			Entry("no healthy condition", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions:         []gardencorev1beta1.Condition{},
				},
			}, HaveOccurred()),
			Entry("no conditions", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("no status", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
			}, HaveOccurred()),
		)
	})

	Context("#CheckManagedResourceProgressing", func() {
		DescribeTable("managedresource",
			func(mr resourcesv1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := health.CheckManagedResourceProgressing(&mr)
				Expect(err).To(matcher)
			},
			Entry("progressing condition not false", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesProgressing,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("progressing condition false", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesProgressing,
							Status: gardencorev1beta1.ConditionFalse,
						},
					},
				},
			}, Not(HaveOccurred())),
			Entry("no progressing condition", resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions:         []gardencorev1beta1.Condition{},
				},
			}, HaveOccurred()),
		)
	})
})
