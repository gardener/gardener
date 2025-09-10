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
	"go.uber.org/mock/gomock"
	authorizationv1 "k8s.io/api/authorization/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/webhook/authorizer"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("WithSelectors", func() {
	Describe("WithSelectorsChecker", func() {
		var (
			ctx = context.Background()
			log = logr.Discard()

			ctrl          *gomock.Controller
			mockClient    *mockclient.MockClient
			fakeClientSet kubernetes.Interface
			fakeClock     *testclock.FakeClock

			checker WithSelectorsChecker
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			DeferCleanup(func() { ctrl.Finish() })

			mockClient = mockclient.NewMockClient(ctrl)
			fakeClock = testclock.NewFakeClock(time.Now())
		})

		JustBeforeEach(func() {
			fakeClock.Step(time.Second)
		})

		Describe("#IsPossible", func() {
			When("Kubernetes version is less then 1.31", func() {
				BeforeEach(func() {
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(mockClient).
						WithVersion("1.30.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
				})

				It("should return false", func() {
					possible, err := checker.IsPossible()
					Expect(err).NotTo(HaveOccurred())
					Expect(possible).To(BeFalse())
				})

				It("should never query the API server to check if the feature gate is turned on", func() {
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)

					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
				})
			})

			When("Kubernetes version is at least 1.34", func() {
				BeforeEach(func() {
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(mockClient).
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
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)

					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
				})
			})

			When("Kubernetes version is between 1.31 and 1.33", func() {
				BeforeEach(func() {
					fakeClientSet = fakekubernetes.NewClientSetBuilder().
						WithClient(mockClient).
						WithVersion("1.33.0").
						Build()
					checker = NewWithSelectorsChecker(ctx, log, fakeClientSet, fakeClock)
				})

				It("should return true when the feature is turned on", func() {
					mockClient.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&authorizationv1.SubjectAccessReview{}), gomock.Any()).Do(func(_ context.Context, _ client.Object, _ ...client.CreateOption) {
						// we do not modify the .spec.resourceAttributes.labelSelector here, hence, it is part of the
						// returned object --> feature gate is turned on
					})

					possible, err := checker.IsPossible()
					Expect(err).NotTo(HaveOccurred())
					Expect(possible).To(BeTrue())
				})

				It("should return false when the feature gate is turned off", func() {
					mockClient.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&authorizationv1.SubjectAccessReview{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
						// we remove the .spec.resourceAttributes.labelSelector here, hence, it is not part of the
						// returned object --> feature gate is turned off
						obj.(*authorizationv1.SubjectAccessReview).Spec.ResourceAttributes.LabelSelector = nil
					})

					possible, err := checker.IsPossible()
					Expect(err).NotTo(HaveOccurred())
					Expect(possible).To(BeFalse())
				})

				It("should cache the result for 10 minutes", func() {
					mockClient.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&authorizationv1.SubjectAccessReview{}), gomock.Any()).Times(1)

					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
				})

				It("should re-check the feature gate after 10 minutes", func() {
					mockClient.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&authorizationv1.SubjectAccessReview{}), gomock.Any()).Times(2)

					_, _ = checker.IsPossible()
					fakeClock.Step(5 * time.Minute)
					_, _ = checker.IsPossible()
					fakeClock.Step(5*time.Minute + time.Second)
					_, _ = checker.IsPossible()
				})
			})
		})
	})
})
