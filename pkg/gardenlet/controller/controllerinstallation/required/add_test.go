// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package required_test

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/required"
)

var _ = Describe("Add", func() {
	var (
		reconciler     *Reconciler
		infrastructure *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		infrastructure = &extensionsv1alpha1.Infrastructure{
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "type",
				},
			},
		}
	})

	Describe("#ObjectPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ObjectPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return true for periodic cache resyncs", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: infrastructure.DeepCopy()})).To(BeTrue())
			})

			It("should return false because object is no extensions object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "123"}}, ObjectNew: infrastructure})).To(BeFalse())
			})

			It("should return false because old object is no extensions object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: infrastructure, ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "123"}}})).To(BeFalse())
			})

			It("should return false because extensions type did not change", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.ResourceVersion = "1"
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeFalse())
			})

			It("should return true because extension type was changed", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.Spec.Type = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})

	Describe("#MapObjectKindToControllerInstallations", func() {
		var (
			ctx              = context.TODO()
			log              = logr.Discard()
			fakeGardenClient client.Client
			fakeSeedClient   client.Client
			mapFn            mapper.MapFunc

			infrastructure2                                                           *extensionsv1alpha1.Infrastructure
			controllerRegistration1, controllerRegistration2, controllerRegistration3 *gardencorev1beta1.ControllerRegistration
			controllerInstallation1, controllerInstallation2, controllerInstallation3 *gardencorev1beta1.ControllerInstallation

			type1, type2 = "foo", "bar"
			seedName     = "seed"
		)

		BeforeEach(func() {
			fakeGardenClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, indexer.ControllerInstallationSeedRefNameIndexerFunc).
				Build()
			fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			mapFn = reconciler.MapObjectKindToControllerInstallations(extensionsv1alpha1.InfrastructureResource, func() client.ObjectList { return &extensionsv1alpha1.InfrastructureList{} })

			reconciler.GardenClient = fakeGardenClient
			reconciler.SeedClient = fakeSeedClient
			reconciler.SeedName = seedName
			reconciler.Lock = &sync.RWMutex{}
			reconciler.KindToRequiredTypes = make(map[string]sets.Set[string])

			controllerRegistration1 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "reg1"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.InfrastructureResource, Type: type1},
					},
				},
			}
			controllerRegistration2 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "reg2"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.InfrastructureResource, Type: type2},
					},
				},
			}
			controllerRegistration3 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "reg3"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.ControlPlaneResource, Type: "foo"},
					},
				},
			}

			controllerInstallation1 = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "inst1"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration1.Name},
					SeedRef:         corev1.ObjectReference{Name: seedName},
				},
			}
			controllerInstallation2 = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "inst2"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration2.Name},
					SeedRef:         corev1.ObjectReference{Name: seedName},
				},
			}
			controllerInstallation3 = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "inst3"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration3.Name},
					SeedRef:         corev1.ObjectReference{Name: seedName},
				},
			}

			infrastructure2 = infrastructure.DeepCopy()
			infrastructure.Name = "infra1"
			infrastructure.Spec.Type = type1
			infrastructure2.Name = "infra2"
			infrastructure2.Spec.Type = type2
		})

		It("should do nothing when there are no infrastructure resources", func() {
			Expect(mapFn(ctx, log, nil, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New[string]()))
		})

		It("should do nothing when there are no controllerregistration resources", func() {
			Expect(fakeGardenClient.Create(ctx, controllerInstallation1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerInstallation2)).To(Succeed())

			Expect(fakeSeedClient.Create(ctx, infrastructure)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, infrastructure2)).To(Succeed())

			Expect(mapFn(ctx, log, nil, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1, type2)))
		})

		It("should do nothing when there are no controllerinstallation resources", func() {
			Expect(fakeGardenClient.Create(ctx, controllerRegistration1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerRegistration2)).To(Succeed())

			Expect(fakeSeedClient.Create(ctx, infrastructure)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, infrastructure2)).To(Succeed())

			Expect(mapFn(ctx, log, nil, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1, type2)))
		})

		It("should return the expected names of controllerinstallations", func() {
			Expect(fakeGardenClient.Create(ctx, controllerRegistration1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerRegistration2)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerRegistration3)).To(Succeed())

			Expect(fakeGardenClient.Create(ctx, controllerInstallation1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerInstallation2)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerInstallation3)).To(Succeed())

			Expect(fakeSeedClient.Create(ctx, infrastructure)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, infrastructure2)).To(Succeed())

			Expect(mapFn(ctx, log, nil, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation1.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation2.Name}},
			))
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1, type2)))
		})
	})
})
