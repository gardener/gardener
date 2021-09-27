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

package controllerregistration

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Controller", func() {
	var (
		seedQueue          *fakeQueue
		controllerRegQueue *fakeQueue
		c                  *Controller
		obj                *gardencorev1beta1.Seed

		seedName = "seed"
	)

	BeforeEach(func() {
		seedQueue = &fakeQueue{}
		controllerRegQueue = &fakeQueue{}
		c = &Controller{
			seedQueue:                       seedQueue,
			controllerRegistrationSeedQueue: controllerRegQueue,
		}

		obj = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{},
		}
	})

	Describe("#seedAdd", func() {
		It("should do nothing because the object key computation fails", func() {
			wrongTypeObj := "foo"

			c.seedAdd(wrongTypeObj, true)

			Expect(seedQueue.Len()).To(BeZero())
			Expect(controllerRegQueue.Len()).To(BeZero())
		})

		It("should add the object to both queues", func() {
			c.seedAdd(obj, true)

			Expect(seedQueue.Len()).To(Equal(1))
			Expect(seedQueue.items[0]).To(Equal(seedName))
			Expect(controllerRegQueue.Len()).To(Equal(1))
			Expect(controllerRegQueue.items[0]).To(Equal(seedName))
		})

		It("should add the object to seed queue only", func() {
			c.seedAdd(obj, false)

			Expect(seedQueue.Len()).To(Equal(1))
			Expect(seedQueue.items[0]).To(Equal(seedName))
			Expect(controllerRegQueue.Len()).To(BeZero())
		})
	})

	Describe("#seedUpdate", func() {
		It("should do nothing because the object key computation fails", func() {
			wrongTypeObj := "foo"

			c.seedUpdate(nil, wrongTypeObj)

			Expect(seedQueue.Len()).To(BeZero())
			Expect(controllerRegQueue.Len()).To(BeZero())
		})

		It("should always add the object to the seed queue", func() {
			oldObj := &gardencorev1beta1.Seed{}

			c.seedUpdate(oldObj, obj)

			Expect(seedQueue.Len()).To(Equal(1))
			Expect(seedQueue.items[0]).To(Equal(seedName))
			Expect(controllerRegQueue.Len()).To(BeZero())
		})

		It("should also add the object to controllerRegistrationQueue if DNS provider changed", func() {
			objWithChangedDNSProvider := obj.DeepCopy()
			objWithChangedDNSProvider.Spec = gardencorev1beta1.SeedSpec{
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{},
				},
			}

			c.seedUpdate(obj, objWithChangedDNSProvider)

			Expect(seedQueue.Len()).To(Equal(1))
			Expect(seedQueue.items[0]).To(Equal(seedName))
			Expect(controllerRegQueue.Len()).To(Equal(1))
			Expect(controllerRegQueue.items[0]).To(Equal(seedName))
		})
	})

	Describe("#seedDelete", func() {
		It("should do nothing because the object key computation fails", func() {
			wrongTypeObj := "foo"

			c.seedDelete(wrongTypeObj)

			Expect(seedQueue.Len()).To(BeZero())
			Expect(controllerRegQueue.Len()).To(BeZero())
		})

		It("should add the object to the queue (tomb stone)", func() {
			obj := cache.DeletedFinalStateUnknown{
				Key: seedName,
			}

			c.seedDelete(obj)

			Expect(seedQueue.Len()).To(Equal(1))
			Expect(seedQueue.items[0]).To(Equal(seedName))
			Expect(controllerRegQueue.Len()).To(Equal(1))
			Expect(controllerRegQueue.items[0]).To(Equal(seedName))
		})

		It("should add the object to the queue", func() {
			c.seedDelete(obj)

			Expect(seedQueue.Len()).To(Equal(1))
			Expect(seedQueue.items[0]).To(Equal(seedName))
			Expect(controllerRegQueue.Len()).To(Equal(1))
			Expect(controllerRegQueue.items[0]).To(Equal(seedName))
		})
	})
})

var _ = Describe("seedReconciler", func() {
	const finalizerName = "core.gardener.cloud/controllerregistration"

	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		reconciler reconcile.Reconciler

		ctx      = context.TODO()
		fakeErr  = fmt.Errorf("fake err")
		seedName = "seed"
		seed     *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		reconciler = NewSeedReconciler(logger.NewNopLogger(), c)
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:            seedName,
				ResourceVersion: "42",
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should return nil because object not found", func() {
			c.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return err because object reading failed", func() {
			c.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(fakeErr)

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(fakeErr))
		})

		Context("deletion timestamp not set", func() {
			BeforeEach(func() {
				c.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Seed) error {
					*obj = *seed
					return nil
				})
			})

			It("should ensure the finalizer (error)", func() {
				errToReturn := apierrors.NewNotFound(schema.GroupResource{}, seedName)

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"]}}`, finalizerName)))
					return errToReturn
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(errToReturn))
			})

			It("should ensure the finalizer (no error)", func() {
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"]}}`, finalizerName)))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("deletion timestamp set", func() {
			BeforeEach(func() {
				now := metav1.Now()
				seed.DeletionTimestamp = &now
				seed.Finalizers = []string{FinalizerName}

				c.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Seed) error {
					*obj = *seed
					return nil
				})
			})

			It("should do nothing because finalizer is not present", func() {
				seed.Finalizers = nil

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because installation list failed", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).Return(fakeErr)

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because installation referencing seed exists", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ControllerInstallationList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ControllerInstallationList{Items: []gardencorev1beta1.ControllerInstallation{
						{
							Spec: gardencorev1beta1.ControllerInstallationSpec{
								SeedRef: corev1.ObjectReference{
									Name: seedName,
								},
							},
						},
					}}).DeepCopyInto(obj)
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(ContainSubstring("cannot remove finalizer")))
			})

			It("should remove the finalizer (error)", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ControllerInstallationList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ControllerInstallationList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return fakeErr
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should remove the finalizer (no error)", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ControllerInstallationList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ControllerInstallationList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
