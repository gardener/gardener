// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/utils"
)

var _ = Describe("Utils", func() {
	Describe("#GetThresholdForCondition", func() {
		It("should return the threshold duration", func() {
			Expect(GetThresholdForCondition([]controllermanagerconfigv1alpha1.ConditionThreshold{{Type: "foo", Duration: metav1.Duration{Duration: time.Second}}}, "foo")).To(Equal(time.Second))
		})

		It("should return 0 because no configuration found for condition type", func() {
			Expect(GetThresholdForCondition(nil, "foo")).To(BeZero())
		})
	})

	Describe("#SetToProgressingOr{Unknown,False}", func() {
		var (
			fakeClock          *testing.FakeClock
			conditionThreshold = time.Second
			condition          gardencorev1beta1.Condition
			conditionType      gardencorev1beta1.ConditionType = "Foo"
			reason                                             = "some-reason"
			message                                            = "some-message"
		)

		BeforeEach(func() {
			fakeClock = &testing.FakeClock{}
			condition = gardencorev1beta1.Condition{Type: conditionType}
		})

		tests := func(
			f func(clock clock.Clock,
				conditionThreshold time.Duration,
				condition gardencorev1beta1.Condition,
				reason, message string,
				codes ...gardencorev1beta1.ErrorCode,
			) gardencorev1beta1.Condition,
			eventualConditionStatus gardencorev1beta1.ConditionStatus,
		) {
			Context("when status was True", func() {
				BeforeEach(func() {
					condition.Status = gardencorev1beta1.ConditionTrue
				})

				It("should set the status to Progressing when conditionThreshold != 0", func() {
					Expect(f(fakeClock, conditionThreshold, condition, reason, message)).To(Equal(gardencorev1beta1.Condition{
						Type:    conditionType,
						Status:  gardencorev1beta1.ConditionProgressing,
						Reason:  reason,
						Message: message,
					}))
				})

				It("should set the status to eventualConditionStatus when conditionThreshold == 0", func() {
					Expect(f(fakeClock, 0, condition, reason, message)).To(Equal(gardencorev1beta1.Condition{
						Type:    conditionType,
						Status:  eventualConditionStatus,
						Reason:  reason,
						Message: message,
					}))
				})
			})

			Context("when status was Progressing", func() {
				BeforeEach(func() {
					condition.Status = gardencorev1beta1.ConditionProgressing
				})

				It("should keep the Progressing status when delta since last transition time within conditionThreshold", func() {
					Expect(f(fakeClock, conditionThreshold, condition, reason, message)).To(Equal(gardencorev1beta1.Condition{
						Type:    conditionType,
						Status:  gardencorev1beta1.ConditionProgressing,
						Reason:  reason,
						Message: message,
					}))
				})

				It("should set the status to eventualConditionStatus when delta since last transition time outside conditionThreshold", func() {
					fakeClock.Step(time.Hour)

					Expect(f(fakeClock, conditionThreshold, condition, reason, message)).To(Equal(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             eventualConditionStatus,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
						LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
						Reason:             reason,
						Message:            message,
					}))
				})

				It("should set the status to eventualConditionStatus when conditionThreshold == 0", func() {
					Expect(f(fakeClock, 0, condition, reason, message)).To(Equal(gardencorev1beta1.Condition{
						Type:    conditionType,
						Status:  eventualConditionStatus,
						Reason:  reason,
						Message: message,
					}))
				})
			})

			Context("when status was False", func() {
				BeforeEach(func() {
					condition.Status = gardencorev1beta1.ConditionFalse
				})

				It("should set the status to eventualConditionStatus", func() {
					Expect(f(fakeClock, conditionThreshold, condition, reason, message)).To(Equal(gardencorev1beta1.Condition{
						Type:    conditionType,
						Status:  eventualConditionStatus,
						Reason:  reason,
						Message: message,
					}))
				})
			})

			Context("when status was Unknown", func() {
				BeforeEach(func() {
					condition.Status = gardencorev1beta1.ConditionUnknown
				})

				It("should keep the eventualConditionStatus status", func() {
					Expect(f(fakeClock, conditionThreshold, condition, reason, message)).To(Equal(gardencorev1beta1.Condition{
						Type:    conditionType,
						Status:  eventualConditionStatus,
						Reason:  reason,
						Message: message,
					}))
				})
			})
		}

		Describe("#SetToProgressingOrUnknown", func() {
			tests(SetToProgressingOrUnknown, gardencorev1beta1.ConditionUnknown)
		})

		Describe("#SetToProgressingOrFalse", func() {
			tests(SetToProgressingOrFalse, gardencorev1beta1.ConditionFalse)
		})
	})

	Describe("#PatchSeedCondition", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client
			condition  gardencorev1beta1.Condition
			seed       *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			log = logr.Discard()
			condition = gardencorev1beta1.Condition{Type: "Foo"}
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: "seed"},
				Status:     gardencorev1beta1.SeedStatus{Conditions: []gardencorev1beta1.Condition{condition}},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Seed{}).Build()
			Expect(fakeClient.Create(ctx, seed)).To(Succeed())
			DeferCleanup(func() {
				Expect(fakeClient.Delete(ctx, seed)).To(Succeed())
			})
		})

		It("should patch the conditions", func() {
			condition.Status = gardencorev1beta1.ConditionFalse

			Expect(PatchSeedCondition(ctx, log, fakeClient.Status(), seed, condition)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			Expect(seed.Status.Conditions).To(ConsistOf(condition))
		})

		It("should not patch the conditions", func() {
			Expect(PatchSeedCondition(ctx, log, fakeClient.Status(), seed, condition)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			Expect(seed.Status.Conditions).To(ConsistOf(condition))
		})
	})
})
