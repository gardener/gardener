// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

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

	Context("#CheckManagedResourcesHonored", func() {
		managedResource := func(name string, annotations map[string]string) resourcesv1alpha1.ManagedResource {
			return resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations},
			}
		}

		DescribeTable("managedresources honored",
			func(managedResources []resourcesv1alpha1.ManagedResource, expectedStatus gardencorev1beta1.ConditionStatus, expectedReason string, messageMatcher types.GomegaMatcher) {
				status, reason, message := health.CheckManagedResourcesHonored(managedResources)
				Expect(status).To(Equal(expectedStatus))
				Expect(reason).To(Equal(expectedReason))
				Expect(message).To(messageMatcher)
			},
			Entry("no managed resources",
				nil,
				gardencorev1beta1.ConditionTrue,
				"AllManagedResourcesActive",
				Equal("No ManagedResources are annotated to be ignored."),
			),
			Entry("managed resource without ignore annotation",
				[]resourcesv1alpha1.ManagedResource{managedResource("foo", nil)},
				gardencorev1beta1.ConditionTrue,
				"AllManagedResourcesActive",
				Equal("No ManagedResources are annotated to be ignored."),
			),
			Entry("managed resource with ignore annotation set to false",
				[]resourcesv1alpha1.ManagedResource{managedResource("foo", map[string]string{resourcesv1alpha1.Ignore: "false"})},
				gardencorev1beta1.ConditionTrue,
				"AllManagedResourcesActive",
				Equal("No ManagedResources are annotated to be ignored."),
			),
			Entry("managed resource with ignore annotation set to a non-boolean value",
				[]resourcesv1alpha1.ManagedResource{managedResource("foo", map[string]string{resourcesv1alpha1.Ignore: "foo"})},
				gardencorev1beta1.ConditionTrue,
				"AllManagedResourcesActive",
				Equal("No ManagedResources are annotated to be ignored."),
			),
			Entry("single managed resource with ignore annotation set to true",
				[]resourcesv1alpha1.ManagedResource{managedResource("foo", map[string]string{resourcesv1alpha1.Ignore: "true"})},
				gardencorev1beta1.ConditionFalse,
				"ManagedResourcesIgnored",
				ContainSubstring("foo"),
			),
			Entry("multiple ignored managed resources are listed alphabetically",
				[]resourcesv1alpha1.ManagedResource{
					managedResource("zeta", map[string]string{resourcesv1alpha1.Ignore: "true"}),
					managedResource("alpha", map[string]string{resourcesv1alpha1.Ignore: "true"}),
					managedResource("mid", map[string]string{resourcesv1alpha1.Ignore: "true"}),
				},
				gardencorev1beta1.ConditionFalse,
				"ManagedResourcesIgnored",
				Equal("Some ManagedResources have been annotated with resources.gardener.cloud/ignore=true, meaning their reconciliation is disabled: alpha, mid, zeta"),
			),
			Entry("mix of ignored and non-ignored managed resources lists only the ignored ones",
				[]resourcesv1alpha1.ManagedResource{
					managedResource("ignored", map[string]string{resourcesv1alpha1.Ignore: "true"}),
					managedResource("active", nil),
					managedResource("disabled-ignore", map[string]string{resourcesv1alpha1.Ignore: "false"}),
				},
				gardencorev1beta1.ConditionFalse,
				"ManagedResourcesIgnored",
				And(ContainSubstring("ignored"), Not(ContainSubstring("active")), Not(ContainSubstring("disabled-ignore"))),
			),
		)
	})
})
