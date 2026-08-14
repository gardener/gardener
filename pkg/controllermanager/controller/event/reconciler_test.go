// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/event"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("event reconciler", func() {
	const ttl = time.Hour

	var (
		fakeClock  *testclock.FakeClock
		fakeClient client.Client
		reconciler reconcile.Reconciler

		event   *eventsv1.Event
		request reconcile.Request
	)

	BeforeEach(func() {
		fakeClock = testclock.NewFakeClock(time.Date(2022, 0, 0, 0, 0, 0, 0, time.UTC))

		event = &eventsv1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			EventTime:  metav1.MicroTime{Time: fakeClock.Now()},
			Regarding:  corev1.ObjectReference{Kind: "Project", APIVersion: "core.gardener.cloud/v1beta1"},
		}
		request = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(event)}

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &Reconciler{
			Clock:  fakeClock,
			Client: fakeClient,
			Config: controllermanagerconfigv1alpha1.EventControllerConfiguration{
				TTLNonShootEvents: &metav1.Duration{Duration: ttl},
			},
		}
	})

	JustBeforeEach(func(ctx SpecContext) {
		Expect(fakeClient.Create(ctx, event)).To(Succeed())
	})

	When("event is already gone", func() {
		JustBeforeEach(func(ctx SpecContext) {
			Expect(fakeClient.Delete(ctx, event)).To(Succeed())
		})

		It("should ignore event", func(ctx SpecContext) {
			Expect(reconciler.Reconcile(ctx, request)).To(BeZero())
		})
	})

	When("event is for shoot", func() {
		BeforeEach(func() {
			event.Regarding.Kind = "Shoot"
		})

		It("should ignore event", func(ctx SpecContext) {
			Expect(reconciler.Reconcile(ctx, request)).To(BeZero())
			Expect(fakeClient.Get(ctx, request.NamespacedName, event)).To(Succeed())
		})
	})

	DescribeTableSubtree("requeue and delete event",
		func(fieldName string, mutate func()) {
			BeforeEach(func() {
				if mutate != nil {
					mutate()
				}
			})

			It(fmt.Sprintf("should delete event after %s + TTL", fieldName), func(ctx SpecContext) {
				Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: ttl}))
				Expect(fakeClient.Get(ctx, request.NamespacedName, event)).To(Succeed(), "event still exists")

				fakeClock.Step(ttl)
				Expect(reconciler.Reconcile(ctx, request)).To(BeZero())
				Expect(fakeClient.Get(ctx, request.NamespacedName, event)).To(BeNotFoundError(), "event is deleted")
			})
		},

		Entry("the event occurred only once", "EventTime", nil),

		Entry("the event occurred multiple times", "Series.LastObservedTime", func() {
			event.EventTime.Time = fakeClock.Now().Add(-2 * ttl)
			event.Series = &eventsv1.EventSeries{
				Count:            2,
				LastObservedTime: metav1.MicroTime{Time: fakeClock.Now()},
			}
		}),

		Entry("no object is referenced", "EventTime", func() {
			event.Regarding = corev1.ObjectReference{}
		}),

		Entry("a non-Gardener object is referenced", "EventTime", func() {
			event.Regarding.APIVersion = "v1"
			event.Regarding.Kind = "Namespace"
		}),

		Entry("events reported via corev1.Event API", "LastTimestamp", func() {
			event = &eventsv1.Event{
				// common fields for both APIs
				ObjectMeta: event.ObjectMeta,
				Regarding:  event.Regarding,

				// fields for events reported via corev1
				DeprecatedFirstTimestamp: metav1.Time{Time: fakeClock.Now().Add(-2 * ttl)},
				DeprecatedLastTimestamp:  metav1.Time{Time: fakeClock.Now()},
				DeprecatedCount:          2,
			}
		}),
	)
})
