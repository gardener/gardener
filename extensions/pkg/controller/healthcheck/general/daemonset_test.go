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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/general"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("DaemonSet", func() {
	const (
		daemonSetName = "test-daemonset"
		namespace     = "test-namespace"
	)

	var (
		ctx     context.Context
		request types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		request = types.NamespacedName{Namespace: namespace, Name: "extension-resource"}
	})

	Describe("SeedDaemonSetHealthChecker", func() {
		var checker *general.SeedDaemonSetHealthChecker

		BeforeEach(func() {
			checker = general.NewSeedDaemonSetHealthChecker(daemonSetName)
			checker.SetLoggerSuffix("test-provider", "test-extension")
		})

		Context("when the daemonset does not exist", func() {
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
				Expect(result.Detail).To(ContainSubstring(daemonSetName))
				Expect(result.Detail).To(ContainSubstring(namespace))
			})
		})

		Context("when the daemonset is healthy", func() {
			BeforeEach(func() {
				ds := newHealthyDaemonSet()
				fakeClient := fake.NewClientBuilder().WithObjects(ds).Build()
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

		Context("when the daemonset has outdated observed generation", func() {
			BeforeEach(func() {
				ds := newHealthyDaemonSet()
				ds.Generation = 2
				ds.Status.ObservedGeneration = 1
				fakeClient := fake.NewClientBuilder().WithObjects(ds).Build()
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

		Context("when the daemonset has not enough scheduled pods", func() {
			BeforeEach(func() {
				ds := newHealthyDaemonSet()
				ds.Status.DesiredNumberScheduled = 3
				ds.Status.CurrentNumberScheduled = 1
				fakeClient := fake.NewClientBuilder().WithObjects(ds).Build()
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

		Context("when the daemonset has misscheduled pods", func() {
			BeforeEach(func() {
				ds := newHealthyDaemonSet()
				ds.Status.NumberMisscheduled = 1
				fakeClient := fake.NewClientBuilder().WithObjects(ds).Build()
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

		Context("when the daemonset has unavailable pods after rollout is complete", func() {
			BeforeEach(func() {
				ds := newHealthyDaemonSet()
				ds.Status.NumberUnavailable = 1
				fakeClient := fake.NewClientBuilder().WithObjects(ds).Build()
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
				Expect(err.Error()).To(ContainSubstring("failed to retrieve DaemonSet"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("ShootDaemonSetHealthChecker", func() {
		var checker *general.ShootDaemonSetHealthChecker

		BeforeEach(func() {
			checker = general.NewShootDaemonSetHealthChecker(daemonSetName)
			checker.SetLoggerSuffix("test-provider", "test-extension")
		})

		Context("when the daemonset does not exist", func() {
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

		Context("when the daemonset is healthy", func() {
			BeforeEach(func() {
				ds := newHealthyDaemonSet()
				fakeClient := fake.NewClientBuilder().WithObjects(ds).Build()
				checker.InjectTargetClient(fakeClient)
			})

			It("should return ConditionTrue", func() {
				result, err := checker.Check(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("when the daemonset is unhealthy", func() {
			BeforeEach(func() {
				ds := newHealthyDaemonSet()
				ds.Generation = 2
				ds.Status.ObservedGeneration = 1
				fakeClient := fake.NewClientBuilder().WithObjects(ds).Build()
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
				Expect(err.Error()).To(ContainSubstring("failed to retrieve DaemonSet"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("DaemonSetIsHealthy", func() {
		It("should return true for a healthy daemonset", func() {
			ds := newHealthyDaemonSet()
			isHealthy, err := general.DaemonSetIsHealthy(ds)
			Expect(err).NotTo(HaveOccurred())
			Expect(isHealthy).To(BeTrue())
		})

		It("should return false for a daemonset with outdated generation", func() {
			ds := newHealthyDaemonSet()
			ds.Generation = 2
			ds.Status.ObservedGeneration = 1
			isHealthy, err := general.DaemonSetIsHealthy(ds)
			Expect(err).To(HaveOccurred())
			Expect(isHealthy).To(BeFalse())
			Expect(err.Error()).To(ContainSubstring("unhealthy"))
		})

		It("should return false for a daemonset with misscheduled pods", func() {
			ds := newHealthyDaemonSet()
			ds.Status.NumberMisscheduled = 2
			isHealthy, err := general.DaemonSetIsHealthy(ds)
			Expect(err).To(HaveOccurred())
			Expect(isHealthy).To(BeFalse())
		})
	})
})

func newHealthyDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-daemonset",
			Namespace:  "test-namespace",
			Generation: 1,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		},
		Status: appsv1.DaemonSetStatus{
			ObservedGeneration:     1,
			DesiredNumberScheduled: 3,
			CurrentNumberScheduled: 3,
			UpdatedNumberScheduled: 3,
			NumberAvailable:        3,
			NumberReady:            3,
		},
	}
}
