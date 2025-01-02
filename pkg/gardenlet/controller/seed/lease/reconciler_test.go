// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	fakerestclient "k8s.io/client-go/rest/fake"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed/lease"
	"github.com/gardener/gardener/pkg/healthz"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("LeaseReconciler", func() {
	var (
		ctx            context.Context
		clock          clock.Clock
		gardenClient   client.Client
		seedRESTClient *fakerestclient.RESTClient
		healthManager  healthz.Manager

		seed              *gardencorev1beta1.Seed
		expectedCondition *gardencorev1beta1.Condition
		expectedLease     *coordinationv1.Lease
		namespace         = "gardener-system-seed-lease"

		request          reconcile.Request
		reconciler       *Reconciler
		controllerConfig gardenletconfigv1alpha1.SeedControllerConfiguration
	)

	BeforeEach(func() {
		ctx = context.Background()
		clock = testclock.NewFakeClock(time.Now().Round(time.Second))

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "apple-seed",
				UID:  "abcdef-foo",
			},
		}
		request = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)}

		renewTime := metav1.NewMicroTime(clock.Now())
		expectedLease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "gardener-system-seed-lease",
				Name:      seed.Name,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Seed",
					Name:       seed.Name,
					UID:        seed.UID,
				}},
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       ptr.To(seed.Name),
				LeaseDurationSeconds: ptr.To[int32](2),
				RenewTime:            &renewTime,
			},
		}

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(seed).WithStatusSubresource(&gardencorev1beta1.Seed{}).Build()
		seedRESTClient = &fakerestclient.RESTClient{
			NegotiatedSerializer: serializer.NewCodecFactory(kubernetes.GardenScheme).WithoutConversion(),
			Resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			},
		}

		controllerConfig = gardenletconfigv1alpha1.SeedControllerConfiguration{
			LeaseResyncSeconds:       ptr.To[int32](2),
			LeaseResyncMissThreshold: ptr.To[int32](10),
		}
	})

	JustBeforeEach(func() {
		healthManager = healthz.NewDefaultHealthz()
		Expect(healthManager.Start(ctx)).To(Succeed())

		reconciler = &Reconciler{
			GardenClient:   gardenClient,
			SeedRESTClient: seedRESTClient,
			Config:         controllerConfig,
			Clock:          clock,
			HealthManager:  healthManager,
			LeaseNamespace: namespace,
		}
	})

	AfterEach(func() {
		if err := gardenClient.Get(ctx, request.NamespacedName, seed); !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())

			if expectedCondition != nil {
				Expect(seed.Status.Conditions).To(ConsistOf(*expectedCondition))
			} else {
				Expect(seed.Status.Conditions).To(BeEmpty())
			}
		}

		lease := &coordinationv1.Lease{}
		err := gardenClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: seed.Name}, lease)
		if expectedLease == nil {
			Expect(err).To(BeNotFoundError())
		} else {
			Expect(err).NotTo(HaveOccurred())
			// if we're not expecting a specific resourceVersion, ignore the one set by the fake client
			if expectedLease.ResourceVersion == "" {
				lease.ResourceVersion = ""
			}
			// fake client returns apiVersion,kind set
			lease.SetGroupVersionKind(schema.GroupVersionKind{})
			Expect(lease).To(DeepEqual(expectedLease))
		}
	})

	It("should do nothing if Seed is gone", func() {
		Expect(gardenClient.Delete(ctx, seed)).To(Succeed())
		expectedLease = nil
		expectedCondition = nil

		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		Expect(healthManager.Get()).To(BeTrue())
	})

	It("should check if LeaseResyncSeconds matches the expectedLease value", func() {
		expectedCondition = gardenletReadyCondition(clock)
		expectedLease.Spec.LeaseDurationSeconds = ptr.To[int32](3)

		reconciler.Config.LeaseResyncSeconds = ptr.To[int32](3)
		request = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)}

		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: 3 * time.Second}))
		Expect(healthManager.Get()).To(BeTrue())
	})

	It("should fail if connection to Seed fails", func() {
		seedRESTClient.Resp.StatusCode = http.StatusInternalServerError
		expectedLease = nil
		expectedCondition = nil

		_, err := reconciler.Reconcile(ctx, request)
		Expect(err).To(HaveOccurred())
		Expect(healthManager.Get()).To(BeFalse())
	})

	Context("failure creating lease", func() {
		BeforeEach(func() {
			gardenClient = failingLeaseClient{gardenClient}
			expectedLease = nil
			expectedCondition = nil
		})

		It("should set health status to false if creating lease fails", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
			Expect(healthManager.Get()).To(BeFalse())
		})
	})

	Context("failure renewing lease", func() {
		BeforeEach(func() {
			// create pre-existing lease
			lease := &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "gardener-system-seed-lease",
					Name:      seed.Name,
				},
			}
			Expect(gardenClient.Create(ctx, lease)).To(Succeed())

			gardenClient = failingLeaseClient{gardenClient}
			expectedLease = lease.DeepCopy()
			expectedCondition = nil
		})

		It("should set health status to false if updating lease fails", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
			Expect(healthManager.Get()).To(BeFalse())
		})
	})

	It("adds GardenletReady condition after renewing lease", func() {
		expectedCondition = gardenletReadyCondition(clock)

		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: 2 * time.Second}))
		Expect(healthManager.Get()).To(BeTrue())
	})

	It("updates GardenletReady condition if it already exists", func() {
		seed.Status.Conditions = []gardencorev1beta1.Condition{{
			Type:               "GardenletReady",
			Status:             "False",
			Reason:             "SomeProblem",
			Message:            "You were probably paged",
			LastTransitionTime: metav1.NewTime(clock.Now().Add(-time.Hour)),
			LastUpdateTime:     metav1.NewTime(clock.Now().Add(-time.Minute)),
		}}
		Expect(gardenClient.Status().Update(ctx, seed)).To(Succeed())

		expectedCondition = gardenletReadyCondition(clock)

		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: 2 * time.Second}))
		Expect(healthManager.Get()).To(BeTrue())
	})
})

// failingLeaseClient returns fake errors for creating and updating leases for testing purposes
type failingLeaseClient struct {
	client.Client
}

func (c failingLeaseClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*coordinationv1.Lease); ok {
		return errors.New("fake")
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c failingLeaseClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*coordinationv1.Lease); ok {
		return errors.New("fake")
	}
	return c.Client.Update(ctx, obj, opts...)
}

func gardenletReadyCondition(clock clock.Clock) *gardencorev1beta1.Condition {
	now := metav1.NewTime(clock.Now().Round(time.Second))
	return &gardencorev1beta1.Condition{
		Type:               "GardenletReady",
		Status:             "True",
		Reason:             "GardenletReady",
		Message:            "Gardenlet is posting ready status.",
		LastTransitionTime: now,
		LastUpdateTime:     now,
	}
}
