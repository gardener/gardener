// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package general_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/general"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("StatefulSet", func() {
	const (
		statefulSetName = "test-statefulset"
		namespace       = "test-namespace"
	)

	var (
		ctx     context.Context
		request types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		request = types.NamespacedName{Namespace: namespace, Name: "extension-resource"}
	})

	Describe("SeedStatefulSetHealthChecker", func() {
		var checker *general.SeedStatefulSetHealthChecker

		BeforeEach(func() {
			checker = general.NewSeedStatefulSetChecker(statefulSetName)
			checker.SetLoggerSuffix("test-provider", "test-extension")
		})

		Context("when the statefulset does not exist", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionFalse with not found detail", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("not found"))
				Expect(result.Detail).To(ContainSubstring(statefulSetName))
				Expect(result.Detail).To(ContainSubstring(namespace))
			})
		})

		Context("when the statefulset is healthy", func() {
			BeforeEach(func() {
				sts := newHealthyStatefulSet()
				fakeClient := fake.NewClientBuilder().WithObjects(sts).Build()
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

		Context("when the statefulset has outdated observed generation", func() {
			BeforeEach(func() {
				sts := newHealthyStatefulSet()
				sts.Generation = 2
				sts.Status.ObservedGeneration = 1
				fakeClient := fake.NewClientBuilder().WithObjects(sts).Build()
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

		Context("when the statefulset does not have enough ready replicas", func() {
			BeforeEach(func() {
				sts := newHealthyStatefulSet()
				sts.Spec.Replicas = ptr.To[int32](3)
				sts.Status.ReadyReplicas = 1
				fakeClient := fake.NewClientBuilder().WithObjects(sts).Build()
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

		Context("when spec.replicas is nil (defaults to 1) and ready replicas is 1", func() {
			BeforeEach(func() {
				sts := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:       statefulSetName,
						Namespace:  namespace,
						Generation: 1,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: nil, // defaults to 1
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
					},
					Status: appsv1.StatefulSetStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      1,
					},
				}
				fakeClient := fake.NewClientBuilder().WithObjects(sts).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return ConditionTrue", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("when spec.replicas is nil (defaults to 1) and ready replicas is 0", func() {
			BeforeEach(func() {
				sts := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:       statefulSetName,
						Namespace:  namespace,
						Generation: 1,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: nil, // defaults to 1
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
					},
					Status: appsv1.StatefulSetStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      0,
					},
				}
				fakeClient := fake.NewClientBuilder().WithObjects(sts).Build()
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

		Context("when the client returns a non-NotFound error", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("internal error")
					},
				}).Build()
				checker.InjectSourceClient(fakeClient)
			})

			It("should return an error", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to retrieve StatefulSet"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("ShootStatefulSetHealthChecker", func() {
		var checker *general.ShootStatefulSetHealthChecker

		BeforeEach(func() {
			checker = general.NewShootStatefulSetChecker(statefulSetName)
			checker.SetLoggerSuffix("test-provider", "test-extension")
		})

		Context("when the statefulset does not exist", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return ConditionFalse with not found detail", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("not found"))
			})
		})

		Context("when the statefulset is healthy", func() {
			BeforeEach(func() {
				sts := newHealthyStatefulSet()
				fakeClient := fake.NewClientBuilder().WithObjects(sts).Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return ConditionTrue", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("when the statefulset is unhealthy", func() {
			BeforeEach(func() {
				sts := newHealthyStatefulSet()
				sts.Generation = 2
				sts.Status.ObservedGeneration = 1
				fakeClient := fake.NewClientBuilder().WithObjects(sts).Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return ConditionFalse", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Detail).To(ContainSubstring("unhealthy"))
			})
		})

		Context("when the client returns a non-NotFound error", func() {
			BeforeEach(func() {
				fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("internal error")
					},
				}).Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return an error", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to retrieve StatefulSet"))
				Expect(result).To(BeNil())
			})
		})
	})
})

func newHealthyStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-statefulset",
			Namespace:  "test-namespace",
			Generation: 1,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To[int32](3),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		},
		Status: appsv1.StatefulSetStatus{
			ObservedGeneration: 1,
			Replicas:           3,
			ReadyReplicas:      3,
			UpdatedReplicas:    3,
			CurrentReplicas:    3,
		},
	}
}
