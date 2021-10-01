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

package controllerutils

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Patch", func() {
	var (
		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake err")

		ctrl   *gomock.Controller
		c      *mockclient.MockClient
		scheme *runtime.Scheme
		obj    *corev1.ServiceAccount
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		obj = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"}}

		scheme = runtime.NewScheme()
		Expect(corescheme.AddToScheme(scheme)).NotTo(HaveOccurred())

		c.EXPECT().Scheme().Return(scheme).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("GetAndCreateOr*Patch", func() {
		testSuite := func(f func(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error), patchType types.PatchType) {
			It("should return an error because reading the object fails", func() {
				c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).Return(fakeErr)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the mutate function returned an error (object found)", func() {
				c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj)

				result, err := f(ctx, c, obj, func() error { return fakeErr })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the mutate function returned an error (object not found)", func() {
				c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

				result, err := f(ctx, c, obj, func() error { return fakeErr })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the create failed", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, obj).Return(fakeErr),
				)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully create the object", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, obj),
				)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because the patch failed", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj),
					test.EXPECTPatch(ctx, c, obj, obj, patchType).Return(fakeErr),
				)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully patch the object", func() {
				objCopy := obj.DeepCopy()
				mutateFn := func(o *corev1.ServiceAccount) func() error {
					return func() error {
						o.Labels = map[string]string{"foo": "bar"}
						return nil
					}
				}
				_ = mutateFn(objCopy)()

				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj),
					test.EXPECTPatch(ctx, c, objCopy, obj, patchType),
				)

				result, err := f(ctx, c, obj, mutateFn(obj))
				Expect(result).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
			})
		}

		Describe("#GetAndCreateOrMergePatch", func() { testSuite(GetAndCreateOrMergePatch, types.MergePatchType) })
		Describe("#GetAndCreateOrStrategicMergePatch", func() { testSuite(GetAndCreateOrStrategicMergePatch, types.StrategicMergePatchType) })
	})

	Describe("CreateOrGetAnd*Patch", func() {
		testSuite := func(f func(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error), patchType types.PatchType) {
			It("should return an error because the mutate function returned an error", func() {
				result, err := f(ctx, c, obj, func() error { return fakeErr })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the create failed", func() {
				c.EXPECT().Create(ctx, obj).Return(fakeErr)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully create the object", func() {
				c.EXPECT().Create(ctx, obj)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because the get failed", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, obj).Return(apierrors.NewAlreadyExists(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).Return(fakeErr),
				)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the patch failed", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, obj).Return(apierrors.NewAlreadyExists(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, objToReturn *corev1.ServiceAccount) error {
						obj.DeepCopyInto(objToReturn)
						return nil
					}),
					test.EXPECTPatch(ctx, c, obj, obj, patchType, fakeErr),
				)

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully patch the object", func() {
				objCopy := obj.DeepCopy()
				mutateFn := func(o *corev1.ServiceAccount) func() error {
					return func() error {
						o.Labels = map[string]string{"foo": "bar"}
						return nil
					}
				}
				_ = mutateFn(objCopy)()

				gomock.InOrder(
					c.EXPECT().Create(ctx, obj).Return(apierrors.NewAlreadyExists(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, objToReturn *corev1.ServiceAccount) error {
						Expect(obj.GetLabels()).To(BeEmpty(), "object should be reset before getting it again")

						obj.DeepCopyInto(objToReturn)
						return nil
					}),
					test.EXPECTPatch(ctx, c, objCopy, obj, patchType),
				)

				result, err := f(ctx, c, obj, mutateFn(obj))
				Expect(result).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
			})
		}

		Describe("#CreateOrGetAndMergePatch", func() { testSuite(CreateOrGetAndMergePatch, types.MergePatchType) })
		Describe("#CreateOrGetAndStrategicMergePatch", func() { testSuite(CreateOrGetAndStrategicMergePatch, types.StrategicMergePatchType) })
	})

	Describe("#TryPatch", func() {
		It("should set state to obj, when conflict occurs", func() {
			s := runtime.NewScheme()
			Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
			objInFakeClient := newInfraObj()
			objInFakeClient.SetResourceVersion("1")
			objInFakeClient.Status.Conditions = []v1beta1.Condition{
				{Type: "Health", Reason: "reason", Message: "messages", Status: "status", LastUpdateTime: metav1.Now()},
			}

			c := fake.NewClientBuilder().WithScheme(s).WithObjects(objInFakeClient).Build()
			infraObj := objInFakeClient.DeepCopy()
			transform := func() error {
				infraState, _ := json.Marshal(state{"someState"})
				infraObj.GetExtensionStatus().SetState(&runtime.RawExtension{Raw: infraState})
				return nil
			}

			u := &conflictErrManager{
				conflictsBeforeUpdate: 2,
				client:                c,
			}

			tryPatchErr := tryPatch(context.Background(), retry.DefaultRetry, c, infraObj, u.patchFunc, transform)
			Expect(tryPatchErr).NotTo(HaveOccurred())

			objFromFakeClient := &extensionsv1alpha1.Infrastructure{}
			err := c.Get(context.Background(), kutil.Key("infraNamespace", "infraName"), objFromFakeClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFromFakeClient).To(Equal(infraObj))
		})
	})
})

func (c *conflictErrManager) patchFunc(ctx context.Context, obj client.Object, patch client.Patch, o ...client.PatchOption) error {
	if c.conflictsBeforeUpdate == c.conflictsOccured {
		return c.client.Status().Patch(ctx, obj, patch, o...)
	}

	c.conflictsOccured++
	return apierrors.NewConflict(schema.GroupResource{}, "", nil)
}
