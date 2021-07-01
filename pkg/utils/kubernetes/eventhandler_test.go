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

package kubernetes_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockpredicate "github.com/gardener/gardener/pkg/mock/controller-runtime/predicate"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/utils/kubernetes/mock"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("eventhandler", func() {
	var (
		ctrl *gomock.Controller

		r        *mockclient.MockReader
		p        *mockpredicate.MockPredicate
		cpf      *mockkubernetes.MockControllerPredicateFactory
		enqueuer *mockkubernetes.MockEnqueuer

		ctx    context.Context
		scheme *runtime.Scheme

		h *ControlledResourceEventHandler

		set   *seedmanagementv1alpha1.ManagedSeedSet
		shoot *gardencorev1beta1.Shoot
		seed  *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		r = mockclient.NewMockReader(ctrl)
		p = mockpredicate.NewMockPredicate(ctrl)
		cpf = mockkubernetes.NewMockControllerPredicateFactory(ctrl)
		enqueuer = mockkubernetes.NewMockEnqueuer(ctrl)

		ctx = context.TODO()
		scheme = runtime.NewScheme()
		Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(seedmanagementv1alpha1.AddToScheme(scheme)).To(Succeed())

		h = &ControlledResourceEventHandler{
			ControllerTypes: []ControllerType{
				{Type: &seedmanagementv1alpha1.ManagedSeedSet{}},
			},
			Ctx:                        ctx,
			Reader:                     r,
			ControllerPredicateFactory: cpf,
			Enqueuer:                   enqueuer,
			Scheme:                     scheme,
			Logger:                     gardenerlogger.NewNopLogger(),
		}

		set = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				UID:       "7657955f-42d0-41a9-bb28-53541454f303",
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name + "-0",
				Namespace:       namespace,
				ResourceVersion: "1",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(set, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedSet")),
				},
			},
		}
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name + "-0",
				ResourceVersion: "1",
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectGetManagedSeedSet = func(found bool, times int) {
			r.EXPECT().Get(ctx, Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedSet{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *seedmanagementv1alpha1.ManagedSeedSet) error {
					if found {
						*s = *set
						return nil
					}
					return apierrors.NewNotFound(seedmanagementv1alpha1.Resource("managedSeedSet"), name)
				},
			).Times(times)
		}
		expectGetShoot = func(found bool, times int) {
			r.EXPECT().Get(ctx, Key(namespace, name+"-0"), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Shoot) error {
					if found {
						*s = *shoot
						return nil
					}
					return apierrors.NewNotFound(gardencorev1beta1.Resource("shoot"), name)
				},
			).Times(times)
		}
	)

	Describe("#OnAdd", func() {
		Context("single controller", func() {
			It("should enqueue the controller if it's found and the predicate returns true", func() {
				expectGetManagedSeedSet(true, 1)
				p.EXPECT().Create(event.CreateEvent{Object: shoot}).Return(true)
				cpf.EXPECT().NewControllerPredicate(set).Return(p)
				enqueuer.EXPECT().Enqueue(set)
				h.OnAdd(shoot)
			})

			It("should not enqueue anything if the object is not a shoot", func() {
				h.OnAdd(42)
			})

			It("should not enqueue anything if the object doesn't have a controller ref", func() {
				shoot.OwnerReferences = nil
				h.OnAdd(shoot)
			})

			It("should not enqueue anything if the controller is not found", func() {
				expectGetManagedSeedSet(false, 1)
				h.OnAdd(shoot)
			})

			It("should not enqueue the controller if it's found but the predicate returns false", func() {
				expectGetManagedSeedSet(true, 1)
				p.EXPECT().Create(event.CreateEvent{Object: shoot}).Return(false)
				cpf.EXPECT().NewControllerPredicate(set).Return(p)
				h.OnAdd(shoot)
			})
		})

		Context("chain of controllers", func() {
			BeforeEach(func() {
				h.ControllerTypes = []ControllerType{
					{
						Type:      &gardencorev1beta1.Shoot{},
						Namespace: pointer.String(gardencorev1beta1constants.GardenNamespace),
						NameFunc:  func(obj client.Object) string { return obj.GetName() },
					},
					{Type: &seedmanagementv1alpha1.ManagedSeedSet{}},
				}
			})

			It("should enqueue the top-most controller if it's found and the predicate returns true", func() {
				expectGetShoot(true, 1)
				expectGetManagedSeedSet(true, 1)
				p.EXPECT().Create(event.CreateEvent{Object: seed}).Return(true)
				cpf.EXPECT().NewControllerPredicate(set).Return(p)
				enqueuer.EXPECT().Enqueue(set)
				h.OnAdd(seed)
			})

			It("should not enqueue anything if the first controller is not found", func() {
				expectGetShoot(false, 1)
				h.OnAdd(seed)
			})

			It("should not enqueue anything if the first controller doesn't have a controller ref", func() {
				shoot.OwnerReferences = nil
				expectGetShoot(true, 1)
				h.OnAdd(seed)
			})

			It("should not enqueue anything if the top-most controller is not found", func() {
				expectGetShoot(true, 1)
				expectGetManagedSeedSet(false, 1)
				h.OnAdd(seed)
			})

			It("should not enqueue the top-most controller if it's found but the predicate returns false", func() {
				expectGetShoot(true, 1)
				expectGetManagedSeedSet(true, 1)
				p.EXPECT().Create(event.CreateEvent{Object: seed}).Return(false)
				cpf.EXPECT().NewControllerPredicate(set).Return(p)
				h.OnAdd(seed)
			})
		})
	})

	Describe("#OnUpdate", func() {
		var (
			newShoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			newShoot = shoot.DeepCopy()
			newShoot.ResourceVersion = "2"
		})

		It("should enqueue the new controller if it's found and the predicate returns true", func() {
			expectGetManagedSeedSet(true, 2)
			p.EXPECT().Update(event.UpdateEvent{ObjectOld: shoot, ObjectNew: newShoot}).Return(true)
			cpf.EXPECT().NewControllerPredicate(set).Return(p)
			enqueuer.EXPECT().Enqueue(set)
			h.OnUpdate(shoot, newShoot)
		})

		It("should enqueue the old controller if it's found, and the predicate returns true", func() {
			newShoot.OwnerReferences = nil
			expectGetManagedSeedSet(true, 1)
			p.EXPECT().Update(event.UpdateEvent{ObjectOld: shoot, ObjectNew: newShoot}).Return(true)
			cpf.EXPECT().NewControllerPredicate(set).Return(p)
			enqueuer.EXPECT().Enqueue(set)
			h.OnUpdate(shoot, newShoot)
		})

		It("should not enqueue anything if either the new or the old object is not a shoot", func() {
			h.OnUpdate(42, shoot)
			h.OnUpdate(shoot, 42)
		})

		It("should not enqueue anything if the resource version hasn't changed", func() {
			h.OnUpdate(shoot, shoot)
		})

		It("should not enqueue anything if the new and old objects don't have a controller ref", func() {
			shoot.OwnerReferences = nil
			newShoot.OwnerReferences = nil
			h.OnUpdate(shoot, newShoot)
		})

		It("should not enqueue anything if the old controller is not found", func() {
			newShoot.OwnerReferences = nil
			expectGetManagedSeedSet(false, 1)
			h.OnUpdate(shoot, newShoot)
		})

		It("should not enqueue the old controller if it's found but the predicate returns false", func() {
			newShoot.OwnerReferences = nil
			expectGetManagedSeedSet(true, 1)
			p.EXPECT().Update(event.UpdateEvent{ObjectOld: shoot, ObjectNew: newShoot}).Return(false)
			cpf.EXPECT().NewControllerPredicate(set).Return(p)
			h.OnUpdate(shoot, newShoot)
		})

		It("should not enqueue anything if the new controller is not found", func() {
			expectGetManagedSeedSet(false, 2)
			h.OnUpdate(shoot, newShoot)
		})

		It("should not enqueue the new controller if it's found but the predicate returns false", func() {
			expectGetManagedSeedSet(true, 2)
			p.EXPECT().Update(event.UpdateEvent{ObjectOld: shoot, ObjectNew: newShoot}).Return(false)
			cpf.EXPECT().NewControllerPredicate(set).Return(p)
			h.OnUpdate(shoot, newShoot)
		})
	})

	Describe("#OnDelete", func() {
		It("should enqueue the controller if it's found and the predicate returns true", func() {
			expectGetManagedSeedSet(true, 1)
			p.EXPECT().Delete(event.DeleteEvent{Object: shoot}).Return(true)
			cpf.EXPECT().NewControllerPredicate(set).Return(p)
			enqueuer.EXPECT().Enqueue(set)
			h.OnDelete(shoot)
		})

		It("should enqueue the controller if it's found and the predicate returns true, with a tombstone", func() {
			expectGetManagedSeedSet(true, 1)
			p.EXPECT().Delete(event.DeleteEvent{Object: shoot}).Return(true)
			cpf.EXPECT().NewControllerPredicate(set).Return(p)
			enqueuer.EXPECT().Enqueue(set)
			h.OnDelete(cache.DeletedFinalStateUnknown{Obj: shoot})
		})

		It("should not enqueue anything if the object is neither a shoot nor a tombstone", func() {
			h.OnDelete(42)
		})

		It("should not enqueue anything if the object is a tombstone of something else than a shoot", func() {
			h.OnDelete(cache.DeletedFinalStateUnknown{Obj: 42})
		})

		It("should not enqueue anything if the object doesn't have a controller ref", func() {
			shoot.OwnerReferences = nil
			h.OnDelete(shoot)
		})

		It("should not enqueue anything if the controller is not found", func() {
			expectGetManagedSeedSet(false, 1)
			h.OnDelete(shoot)
		})

		It("should not enqueue the controller if it's found but the predicate returns false", func() {
			expectGetManagedSeedSet(true, 1)
			p.EXPECT().Delete(event.DeleteEvent{Object: shoot}).Return(false)
			cpf.EXPECT().NewControllerPredicate(set).Return(p)
			h.OnDelete(shoot)
		})
	})
})
