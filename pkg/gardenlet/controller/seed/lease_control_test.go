// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seed_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	fakerestclient "k8s.io/client-go/rest/fake"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("LeaseReconciler", func() {
	var (
		ctx            context.Context
		log            logrus.FieldLogger
		now            metav1.Time
		nowFunc        func() metav1.Time
		c              client.Client
		seedRESTClient *fakerestclient.RESTClient
		healthManager  healthz.Manager

		seed              *gardencorev1beta1.Seed
		expectedCondition *gardencorev1beta1.Condition
		expectedLease     *coordinationv1.Lease

		request    reconcile.Request
		reconciler reconcile.Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.NewNopLogger()

		now = metav1.NewTime(time.Now().Round(time.Second))
		nowFunc = func() metav1.Time { return now }

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "apple-seed",
				UID:  "abcdef-foo",
			},
		}
		request = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)}

		microNow := metav1.NewMicroTime(now.Time)
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
				HolderIdentity:       pointer.String(seed.Name),
				LeaseDurationSeconds: pointer.Int32(2),
				RenewTime:            &microNow,
			},
		}

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(seed).Build()
		seedRESTClient = &fakerestclient.RESTClient{
			NegotiatedSerializer: serializer.NewCodecFactory(kubernetes.GardenScheme).WithoutConversion(),
			Resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			},
		}
	})

	JustBeforeEach(func() {
		fakeClientSet := fakeclientset.NewClientSetBuilder().WithClient(c).Build()
		fakeClientMap := fakeclientmap.NewClientMap().
			AddClient(keys.ForGarden(), fakeClientSet).
			AddClient(keys.ForSeed(seed), fakeclientset.NewClientSetBuilder().WithRESTClient(seedRESTClient).Build())

		healthManager = healthz.NewDefaultHealthz()
		healthManager.Start()

		reconciler = NewLeaseReconciler(fakeClientMap, log, healthManager, nowFunc)
	})

	AfterEach(func() {
		if err := c.Get(ctx, request.NamespacedName, seed); !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())

			if expectedCondition != nil {
				Expect(seed.Status.Conditions).To(ConsistOf(*expectedCondition))
			} else {
				Expect(seed.Status.Conditions).To(BeEmpty())
			}
		}

		lease := &coordinationv1.Lease{}
		err := c.Get(ctx, client.ObjectKey{Namespace: "gardener-system-seed-lease", Name: seed.Name}, lease)
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
		Expect(c.Delete(ctx, seed)).To(Succeed())
		expectedLease = nil
		expectedCondition = nil

		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
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
			c = failingLeaseClient{c}
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
			Expect(c.Create(ctx, lease)).To(Succeed())

			c = failingLeaseClient{c}
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
		expectedCondition = &gardencorev1beta1.Condition{
			Type:               "GardenletReady",
			Status:             "True",
			Reason:             "GardenletReady",
			Message:            "Gardenlet is posting ready status.",
			LastTransitionTime: now,
			LastUpdateTime:     now,
		}

		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: 2 * time.Second}))
		Expect(healthManager.Get()).To(BeTrue())
	})

	It("updates GardenletReady condition if it already exists", func() {
		seed.Status.Conditions = []gardencorev1beta1.Condition{{
			Type:               "GardenletReady",
			Status:             "False",
			Reason:             "SomeProblem",
			Message:            "You were probably paged",
			LastTransitionTime: metav1.NewTime(now.Add(-time.Hour)),
			LastUpdateTime:     metav1.NewTime(now.Add(-time.Minute)),
		}}
		Expect(c.Status().Update(ctx, seed)).To(Succeed())

		expectedCondition = &gardencorev1beta1.Condition{
			Type:               "GardenletReady",
			Status:             "True",
			Reason:             "GardenletReady",
			Message:            "Gardenlet is posting ready status.",
			LastTransitionTime: now,
			LastUpdateTime:     now,
		}

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
		return fmt.Errorf("fake")
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c failingLeaseClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*coordinationv1.Lease); ok {
		return fmt.Errorf("fake")
	}
	return c.Client.Update(ctx, obj, opts...)
}
