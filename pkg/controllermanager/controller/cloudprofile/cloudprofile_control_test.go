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

package cloudprofile

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Controller", func() {
	var (
		ctx  = context.TODO()
		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Event handlers", func() {
		var (
			controller  *Controller
			clientCache *mockcache.MockCache

			cloudProfileName string
		)

		BeforeEach(func() {
			clientCache = mockcache.NewMockCache(ctrl)

			clientCache.EXPECT().GetInformer(ctx, &gardencorev1beta1.CloudProfile{}).DoAndReturn(
				func(_ context.Context, obj runtime.Object) (cache.Informer, error) {
					return gardencoreinformers.NewCloudProfileInformer(nil, 0, nil), nil
				},
			)

			var err error
			controller, err = NewCloudProfileController(ctx, fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForGarden(), fakeclientset.NewClientSetBuilder().WithCache(clientCache).Build()).Build(), &record.FakeRecorder{}, logger.NewNopLogger())
			Expect(err).To(Not(HaveOccurred()))

			cloudProfileName = "test-cloudprofile"
		})

		Describe("#cloudProfileAdd", func() {
			It("should do nothing because the object key computation fails", func() {
				obj := "foo"

				controller.cloudProfileAdd(obj)

				Expect(controller.cloudProfileQueue.Len()).To(BeZero())
			})

			It("should add the object to the queue", func() {
				obj := &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: cloudProfileName,
					},
				}

				controller.cloudProfileAdd(obj)

				Expect(controller.cloudProfileQueue.Len()).To(Equal(1))
				item, _ := controller.cloudProfileQueue.Get()
				Expect(item).To(Equal(cloudProfileName))
			})

			It("should add the object to the queue", func() {
				obj := &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: cloudProfileName,
					},
				}

				controller.cloudProfileAdd(obj)

				Expect(controller.cloudProfileQueue.Len()).To(Equal(1))
				item, _ := controller.cloudProfileQueue.Get()
				Expect(item).To(Equal(cloudProfileName))
			})
		})

		Describe("#cloudProfileUpdate", func() {
			It("should do nothing because the object key computation fails", func() {
				controller.cloudProfileUpdate(nil, nil)

				Expect(controller.cloudProfileQueue.Len()).To(BeZero())
			})

			It("should add the object to the queue", func() {
				obj := &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: cloudProfileName,
					},
				}

				controller.cloudProfileUpdate(nil, obj)

				Expect(controller.cloudProfileQueue.Len()).To(Equal(1))
				item, _ := controller.cloudProfileQueue.Get()
				Expect(item).To(Equal(cloudProfileName))
			})
		})
	})

	Describe("cloudProfileReconciler", func() {
		const finalizerName = gardencorev1beta1.GardenerName

		var (
			cloudProfileName string
			fakeErr          error
			reconciler       reconcile.Reconciler
			cloudProfile     *gardencorev1beta1.CloudProfile
		)

		BeforeEach(func() {
			cloudProfileName = "test-cloudprofile"
			fakeErr = fmt.Errorf("fake err")
			reconciler = NewCloudProfileReconciler(logger.NewNopLogger(), c, &record.FakeRecorder{})
			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:            cloudProfileName,
					ResourceVersion: "42",
				},
			}
		})

		It("should return nil because object not found", func() {
			c.EXPECT().Get(ctx, kutil.Key(cloudProfileName), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return err because object reading failed", func() {
			c.EXPECT().Get(ctx, kutil.Key(cloudProfileName), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).Return(fakeErr)

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(fakeErr))
		})

		Context("when deletion timestamp not set", func() {
			BeforeEach(func() {
				c.EXPECT().Get(ctx, kutil.Key(cloudProfileName), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile) error {
					*obj = *cloudProfile
					return nil
				})
			})

			It("should ensure the finalizer (error)", func() {
				errToReturn := apierrors.NewNotFound(schema.GroupResource{}, cloudProfileName)

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"]}}`, finalizerName)))
					return errToReturn
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(err))
			})

			It("should ensure the finalizer (no error)", func() {
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"]}}`, finalizerName)))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when deletion timestamp set", func() {
			BeforeEach(func() {
				now := metav1.Now()
				cloudProfile.DeletionTimestamp = &now
				cloudProfile.Finalizers = []string{finalizerName}

				c.EXPECT().Get(ctx, kutil.Key(cloudProfileName), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile) error {
					*obj = *cloudProfile
					return nil
				})
			})

			It("should do nothing because finalizer is not present", func() {
				cloudProfile.Finalizers = nil

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because Shoot referencing CloudProfile exists", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: "test-namespace"},
							Spec: gardencorev1beta1.ShootSpec{
								CloudProfileName: cloudProfileName,
							},
						},
					}}).DeepCopyInto(obj)
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(ContainSubstring("Cannot delete CloudProfile")))
			})

			It("should remove the finalizer (error)", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return fakeErr
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should remove the finalizer (no error)", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("migrateMachineImageVersionCRISupport", func() {
		var cloudProfile *gardencorev1beta1.CloudProfile
		var cloudProfileName = "test-cloudprofile"

		BeforeEach(func() {
			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:            cloudProfileName,
					ResourceVersion: "42",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineImages: []gardencorev1beta1.MachineImage{
						{
							Name: "test-image-without-cri",
							Versions: []gardencorev1beta1.MachineImageVersion{
								{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}},
							},
						},
					},
				},
			}
		})

		It("should add `docker` to the list of supported Container Runtimes if it is `nil`", func() {
			migrationHappened := migrateMachineImageVersionCRISupport(cloudProfile)

			Expect(migrationHappened).To(BeTrue())
			Expect(len(cloudProfile.Spec.MachineImages[0].Versions[0].CRI)).To(Equal(1))
			Expect(cloudProfile.Spec.MachineImages[0].Versions[0].CRI[0].Name).To(Equal(gardencorev1beta1.CRINameDocker))
		})

		It("should add `docker` to the list of supported Container Runtimes if it is not in the list yet", func() {
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}}

			migrationHappened := migrateMachineImageVersionCRISupport(cloudProfile)

			Expect(migrationHappened).To(BeTrue())
			Expect(len(cloudProfile.Spec.MachineImages[0].Versions[0].CRI)).To(Equal(2))
			Expect(cloudProfile.Spec.MachineImages[0].Versions[0].CRI).To(ContainElement(gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameDocker}))
		})

		It("should not indicate a migration happened when `docker` is already in the list of supported Container Runtimes", func() {
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}, {Name: gardencorev1beta1.CRINameDocker}}

			migrationHappened := migrateMachineImageVersionCRISupport(cloudProfile)

			Expect(migrationHappened).To(BeFalse())
		})
	})
})
