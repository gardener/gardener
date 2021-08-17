// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ExtensionCheckReconciler", func() {
	const seedName = "test"

	var (
		ctx                     context.Context
		log                     logrus.FieldLogger
		c                       client.Client
		seed                    *gardencorev1beta1.Seed
		controllerInstallations []*gardencorev1beta1.ControllerInstallation
		expectedCondition       gardencorev1beta1.Condition
		now                     metav1.Time
		nowFunc                 func() metav1.Time

		reconciler reconcile.Reconciler
		request    reconcile.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.NewNopLogger()
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: seedName},
		}
		request = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)}

		now = metav1.NewTime(time.Now().Round(time.Second))
		nowFunc = func() metav1.Time { return now }

		expectedCondition = gardencorev1beta1.Condition{
			Type:               "ExtensionsReady",
			Status:             "True",
			LastTransitionTime: now,
			LastUpdateTime:     now,
			Reason:             "AllExtensionsReady",
			Message:            "All extensions installed into the seed cluster are ready and healthy.",
		}

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(seed).Build()
		fakeClientSet := fakeclientset.NewClientSetBuilder().WithClient(c).Build()
		fakeClientMap := fakeclientmap.NewClientMap().AddClient(keys.ForGarden(), fakeClientSet)
		reconciler = NewExtensionCheckReconciler(fakeClientMap, log, nowFunc)
	})

	JustBeforeEach(func() {
		for _, obj := range controllerInstallations {
			Expect(c.Create(ctx, obj)).To(Succeed())
		}
	})

	AfterEach(func() {
		if err := c.Get(ctx, request.NamespacedName, seed); !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
			Expect(seed.Status.Conditions).To(ConsistOf(expectedCondition))
		}
	})

	It("should do nothing if Seed is gone", func() {
		Expect(c.Delete(ctx, seed)).To(Succeed())
		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
	})

	Context("no ControllerInstallations exist", func() {
		It("should set ExtensionsReady to True (AllExtensionsReady)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})
	})

	Context("all ControllerInstallations are not installed", func() {
		BeforeEach(func() {
			expectedCondition.Status = gardencorev1beta1.ConditionFalse
			expectedCondition.Reason = "NotAllExtensionsInstalled"
			expectedCondition.Message = `Some extensions are not installed: map[foo-1:extension was not yet installed foo-3:extension was not yet installed]`

			c1 := &gardencorev1beta1.ControllerInstallation{}
			c1.SetName("foo-1")
			c1.Spec.SeedRef.Name = seedName

			c2 := c1.DeepCopy()
			c2.SetName("foo-2")
			c2.Spec.SeedRef.Name = "not-seed-2"

			c3 := c1.DeepCopy()
			c3.SetName("foo-3")

			controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2, c3}
		})

		It("should set ExtensionsReady to False (NotAllExtensionsInstalled)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})
	})

	Context("all ControllerInstallations valid, installed and healthy", func() {
		BeforeEach(func() {
			c1 := &gardencorev1beta1.ControllerInstallation{}
			c1.SetName("foo-1")
			c1.Spec.SeedRef.Name = seedName
			c1.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
				{Type: "RandomType", Status: gardencorev1beta1.ConditionTrue},
				{Type: "AnotherRandomType", Status: gardencorev1beta1.ConditionFalse},
			}

			c2 := c1.DeepCopy()
			c2.SetName("foo-2")

			controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2}
		})

		It("should set ExtensionsReady to True (AllExtensionsReady)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})

		It("should update ExtensionsReady condition if it already exists", func() {
			existingCondition := gardencorev1beta1.Condition{
				Type:               "ExtensionsReady",
				Status:             gardencorev1beta1.ConditionFalse,
				Reason:             "NotAllExtensionsInstalled",
				Message:            `Some extensions are not installed: map[foo-1:extension was not yet installed foo-3:extension was not yet installed]`,
				LastTransitionTime: metav1.NewTime(now.Add(-time.Minute)),
				LastUpdateTime:     metav1.NewTime(now.Add(-time.Minute)),
			}
			seed.Status.Conditions = []gardencorev1beta1.Condition{existingCondition}
			Expect(c.Status().Update(ctx, seed))

			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})
	})

	Context("one ControllerInstallations is invalid", func() {
		BeforeEach(func() {
			expectedCondition.Status = gardencorev1beta1.ConditionFalse
			expectedCondition.Reason = "NotAllExtensionsValid"
			expectedCondition.Message = `Some extensions are not valid: map[foo-2:]`

			c1 := &gardencorev1beta1.ControllerInstallation{}
			c1.SetName("foo-1")
			c1.Spec.SeedRef.Name = seedName
			c1.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
				{Type: "RandomType", Status: gardencorev1beta1.ConditionTrue},
				{Type: "AnotherRandomType", Status: gardencorev1beta1.ConditionFalse},
			}

			c2 := c1.DeepCopy()
			c2.SetName("foo-2")
			c2.Status.Conditions[0].Status = gardencorev1beta1.ConditionFalse

			controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2}
		})

		It("should set ExtensionsReady to False (NotAllExtensionsValid)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})
	})

	Context("one ControllerInstallation is not installed", func() {
		BeforeEach(func() {
			expectedCondition.Status = gardencorev1beta1.ConditionFalse
			expectedCondition.Reason = "NotAllExtensionsInstalled"
			expectedCondition.Message = `Some extensions are not installed: map[foo-2:]`

			c1 := &gardencorev1beta1.ControllerInstallation{}
			c1.SetName("foo-1")
			c1.Spec.SeedRef.Name = seedName
			c1.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
				{Type: "RandomType", Status: gardencorev1beta1.ConditionTrue},
				{Type: "AnotherRandomType", Status: gardencorev1beta1.ConditionFalse},
			}

			c2 := c1.DeepCopy()
			c2.SetName("foo-2")
			c2.Status.Conditions[1].Status = gardencorev1beta1.ConditionFalse

			controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2}
		})

		It("should set ExtensionsReady to False (NotAllExtensionsInstalled)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})
	})

	Context("one ControllerInstallation is not healthy", func() {
		BeforeEach(func() {
			expectedCondition.Status = gardencorev1beta1.ConditionFalse
			expectedCondition.Reason = "NotAllExtensionsHealthy"
			expectedCondition.Message = `Some extensions are not healthy: map[foo-2:]`

			c1 := &gardencorev1beta1.ControllerInstallation{}
			c1.SetName("foo-1")
			c1.Spec.SeedRef.Name = seedName
			c1.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
				{Type: "RandomType", Status: gardencorev1beta1.ConditionTrue},
				{Type: "AnotherRandomType", Status: gardencorev1beta1.ConditionFalse},
			}

			c2 := c1.DeepCopy()
			c2.SetName("foo-2")
			c2.Status.Conditions[2].Status = gardencorev1beta1.ConditionFalse

			controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2}
		})

		It("should set ExtensionsReady to False (NotAllExtensionsHealthy)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})
	})
})
