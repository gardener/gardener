// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/event"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("eventReconciler", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		fakeClock  *testclock.FakeClock

		shootEventName                 = "shootEvent-test"
		nonShootEventName              = "nonShootEvent-test"
		eventWithoutInvolvedObjectName = "eventWithoutInvolvedObject-test"
		nonGardenerAPIGroupEventName   = "nonGardenerAPIGroupEvent-test"

		ttl = &metav1.Duration{Duration: 1 * time.Hour}

		reconciler                 reconcile.Reconciler
		shootEvent                 *corev1.Event
		nonShootEvent              *corev1.Event
		nonGardenerAPIGroupEvent   *corev1.Event
		eventWithoutInvolvedObject *corev1.Event
		cfg                        controllermanagerconfigv1alpha1.EventControllerConfiguration
	)

	BeforeEach(func() {
		ctx = context.TODO()

		fakeNow := time.Date(2022, 0, 0, 0, 0, 0, 0, time.UTC)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeClock = testclock.NewFakeClock(fakeNow)

		shootEvent = &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: shootEventName},
			LastTimestamp:  metav1.Time{Time: fakeNow},
			InvolvedObject: corev1.ObjectReference{Kind: "Shoot", APIVersion: "core.gardener.cloud/v1beta1"},
		}
		nonShootEvent = &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: nonShootEventName},
			LastTimestamp:  metav1.Time{Time: fakeNow},
			InvolvedObject: corev1.ObjectReference{Kind: "Project", APIVersion: "core.gardener.cloud/v1beta1"},
		}
		eventWithoutInvolvedObject = &corev1.Event{
			ObjectMeta:    metav1.ObjectMeta{Name: eventWithoutInvolvedObjectName},
			LastTimestamp: metav1.Time{Time: fakeNow},
		}
		nonGardenerAPIGroupEvent = &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: nonGardenerAPIGroupEventName},
			LastTimestamp:  metav1.Time{Time: fakeNow},
			InvolvedObject: corev1.ObjectReference{Kind: "Shoot", APIVersion: "v1"},
		}

		cfg = controllermanagerconfigv1alpha1.EventControllerConfiguration{
			TTLNonShootEvents: ttl,
		}

		reconciler = &Reconciler{Clock: fakeClock, Client: fakeClient, Config: cfg}
	})

	It("should return nil because object not found", func() {
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(nonShootEvent), &corev1.Event{})).To(BeNotFoundError())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: nonShootEventName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("shoot events", func() {
		It("should ignore them", func() {
			Expect(fakeClient.Create(ctx, shootEvent)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shootEvent), &corev1.Event{})).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shootEventName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("non-shoot events", func() {
		Context("ttl is not yet reached", func() {
			It("should requeue non-shoot events", func() {
				Expect(fakeClient.Create(ctx, nonShootEvent)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: nonShootEventName}})
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: ttl.Duration}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should requeue events with an empty involvedObject", func() {
				Expect(fakeClient.Create(ctx, eventWithoutInvolvedObject)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: eventWithoutInvolvedObjectName}})
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: ttl.Duration}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should requeue events with non Gardener APIGroup", func() {
				Expect(fakeClient.Create(ctx, nonGardenerAPIGroupEvent)).To(Succeed())
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: nonGardenerAPIGroupEventName}})
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: ttl.Duration}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("ttl is reached", func() {
			BeforeEach(func() {
				fakeClock.Step(ttl.Duration)
				reconciler = &Reconciler{Clock: fakeClock, Client: fakeClient, Config: cfg}

				Expect(fakeClient.Create(ctx, nonShootEvent)).To(Succeed())
			})

			It("should delete the event", func() {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: nonShootEventName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(nonShootEvent), &corev1.Event{})).To(BeNotFoundError())
			})
		})
	})
})
