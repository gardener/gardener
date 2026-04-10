// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authorizer_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/webhook/authorizer"
)

var _ = Describe("WithSelectors", func() {
	Describe("WithSelectorsChecker", func() {
		var (
			ctx = context.Background()
			log = logr.Discard()

			fakeClient    client.Client
			fakeClientSet kubernetes.Interface
			fakeClock     *testclock.FakeClock

			checker WithSelectorsChecker
		)

		BeforeEach(func() {
			fakeClock = testclock.NewFakeClock(time.Now())
		})

		JustBeforeEach(func() {
			fakeClock.Step(time.Second)
		})

		Describe("#IsPossible", func() {
			When("Kubernetes version is at least 1.34", func() {
				BeforeEach(func() {
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()).
						WithVersion("1.34.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
				})

				It("should return true", func() {
					possible, err := checker.IsPossible()
					Expect(err).NotTo(HaveOccurred())
					Expect(possible).To(BeTrue())
				})

				It("should never query the API server to check if the feature gate is turned on", func() {
					createCalls := 0
					interceptedClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							createCalls++
							return c.Create(ctx, obj, opts...)
						},
					}).Build()
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(interceptedClient).
						WithVersion("1.34.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)

					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()

					Expect(createCalls).To(Equal(0))
				})
			})

			When("Kubernetes version is between 1.31 and 1.33", func() {
				BeforeEach(func() {
					fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(fakeClient).
						WithVersion("1.33.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
				})

				It("should return true when the feature is turned on", func() {
					// we do not modify the .spec.resourceAttributes.labelSelector here, hence, it is part of the
					// returned object --> feature gate is turned on
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
					fakeClock.Step(time.Second)

					possible, err := checker.IsPossible()
					Expect(err).NotTo(HaveOccurred())
					Expect(possible).To(BeTrue())
				})

				It("should return false when the feature gate is turned off", func() {
					interceptedClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							// we remove the .spec.resourceAttributes.labelSelector here, hence, it is not part of the
							// returned object --> feature gate is turned off
							obj.(*authorizationv1.SubjectAccessReview).Spec.ResourceAttributes.LabelSelector = nil
							return c.Create(ctx, obj, opts...)
						},
					}).Build()
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(interceptedClient).
						WithVersion("1.33.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
					fakeClock.Step(time.Second)

					possible, err := checker.IsPossible()
					Expect(err).NotTo(HaveOccurred())
					Expect(possible).To(BeFalse())
				})

				It("should cache the result for 10 minutes", func() {
					createCalls := 0

					interceptedClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							createCalls++
							return c.Create(ctx, obj, opts...)
						},
					}).Build()
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(interceptedClient).
						WithVersion("1.33.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
					fakeClock.Step(time.Second)

					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()

					Expect(createCalls).To(Equal(1))
				})

				It("should re-check the feature gate after 10 minutes", func() {
					createCalls := 0

					interceptedClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							createCalls++
							return c.Create(ctx, obj, opts...)
						},
					}).Build()
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(interceptedClient).
						WithVersion("1.33.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
					fakeClock.Step(time.Second)

					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5*time.Minute + time.Second)
					_, _ = checker.IsPossible()

					Expect(createCalls).To(Equal(2))
				})
			})
		})
	})
})
