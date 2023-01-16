// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerregistrationfinalizer_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerregistrationfinalizer"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("ControllerRegistration", func() {
	var (
		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake err")

		ctrl *gomock.Controller
		c    *mockclient.MockClient

		controllerRegistrationName = "controllerRegistration"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Reconciler", func() {
		const finalizerName = "core.gardener.cloud/controllerregistration"

		var (
			reconciler             *Reconciler
			controllerRegistration *gardencorev1beta1.ControllerRegistration
		)

		BeforeEach(func() {
			reconciler = &Reconciler{Client: c}
			controllerRegistration = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:            controllerRegistrationName,
					ResourceVersion: "42",
				},
			}
		})

		It("should return nil because object not found", func() {
			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return err because object reading failed", func() {
			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(fakeErr)

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(fakeErr))
		})

		Context("deletion timestamp not set", func() {
			BeforeEach(func() {
				c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.ControllerRegistration, _ ...client.GetOption) error {
					*obj = *controllerRegistration
					return nil
				})
			})

			It("should ensure the finalizer (error)", func() {
				errToReturn := apierrors.NewNotFound(schema.GroupResource{}, controllerRegistrationName)

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
					return errToReturn
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(err))
			})

			It("should ensure the finalizer (no error)", func() {
				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("deletion timestamp set", func() {
			BeforeEach(func() {
				now := metav1.Now()
				controllerRegistration.DeletionTimestamp = &now
				controllerRegistration.Finalizers = []string{FinalizerName}

				c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.ControllerRegistration, _ ...client.GetOption) error {
					*obj = *controllerRegistration
					return nil
				})
			})

			It("should do nothing because finalizer is not present", func() {
				controllerRegistration.Finalizers = nil

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because installation list failed", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).Return(fakeErr)

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because installation referencing controllerRegistration exists", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ControllerInstallationList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ControllerInstallationList{Items: []gardencorev1beta1.ControllerInstallation{
						{
							Spec: gardencorev1beta1.ControllerInstallationSpec{
								RegistrationRef: corev1.ObjectReference{
									Name: controllerRegistrationName,
								},
							},
						},
					}}).DeepCopyInto(obj)
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(ContainSubstring("cannot remove finalizer")))
			})

			It("should remove the finalizer (error)", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ControllerInstallationList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ControllerInstallationList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return fakeErr
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should remove the finalizer (no error)", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallationList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ControllerInstallationList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ControllerInstallationList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerRegistrationName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
