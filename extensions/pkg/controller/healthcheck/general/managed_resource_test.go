// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package general_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/general"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var _ = Describe("ManagedResource", func() {
	const (
		managedResourceName = "test-managed-resource"
		namespace           = "test-namespace"
	)

	var (
		ctx     context.Context
		request types.NamespacedName
		scheme  *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		request = types.NamespacedName{Namespace: namespace, Name: "extension-resource"}

		scheme = runtime.NewScheme()
		Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	Describe("ManagedResourceHealthChecker", func() {
		var checker *general.ManagedResourceHealthChecker

		BeforeEach(func() {
			checker = general.CheckManagedResource(managedResourceName)
			checker.SetLoggerSuffix("test-provider", "test-extension")
		})

		Context("when the managed resource does not exist", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse with not found detail", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("not found"))
				Expect(result.Detail).To(ContainSubstring(managedResourceName))
				Expect(result.Detail).To(ContainSubstring(namespace))
			})
		})

		Context("when the managed resource is healthy", func() {
			BeforeEach(func() {
				mr := newHealthyManagedResource(managedResourceName, namespace)
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionTrue", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
				Expect(result.Detail).To(BeEmpty())
			})
		})

		Context("when the managed resource has outdated observed generation", func() {
			BeforeEach(func() {
				mr := newHealthyManagedResource(managedResourceName, namespace)
				mr.Generation = 2
				mr.Status.ObservedGeneration = 1
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the managed resource is missing the ResourcesApplied condition", func() {
			BeforeEach(func() {
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the managed resource has ResourcesApplied=False", func() {
			BeforeEach(func() {
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:    resourcesv1alpha1.ResourcesApplied,
								Status:  gardencorev1beta1.ConditionFalse,
								Message: "apply failed",
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the managed resource is missing the ResourcesHealthy condition", func() {
			BeforeEach(func() {
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the managed resource has ResourcesHealthy=False", func() {
			BeforeEach(func() {
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:    resourcesv1alpha1.ResourcesHealthy,
								Status:  gardencorev1beta1.ConditionFalse,
								Message: "deployment default/nginx is not healthy",
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the managed resource has empty conditions", func() {
			BeforeEach(func() {
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions:         []gardencorev1beta1.Condition{},
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse because required conditions are missing", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the unhealthy managed resource matches the configuration problem regex", func() {
			BeforeEach(func() {
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:    resourcesv1alpha1.ResourcesApplied,
								Status:  gardencorev1beta1.ConditionFalse,
								Message: "Error during apply of object default/test is invalid: some field is wrong",
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse with ErrorConfigurationProblem code", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Codes).To(ContainElement(gardencorev1beta1.ErrorConfigurationProblem))
			})
		})

		Context("when the unhealthy managed resource does not match the configuration problem regex", func() {
			BeforeEach(func() {
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:    resourcesv1alpha1.ResourcesApplied,
								Status:  gardencorev1beta1.ConditionFalse,
								Message: "some generic error occurred",
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).WithStatusSubresource(mr).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse without ErrorConfigurationProblem code", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Codes).To(BeEmpty())
			})
		})

		Context("when the client returns a non-NotFound error", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("internal error")
					},
				}).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return an error", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unable to retrieve managed resource"))
				Expect(result).To(BeNil())
			})
		})
	})
})

func newHealthyManagedResource(name, namespace string) *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
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
	}
}
