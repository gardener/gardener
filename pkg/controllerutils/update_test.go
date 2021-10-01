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

package controllerutils

import (
	"context"
	"encoding/json"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	autoscalerv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("utils", func() {
	Describe("#TypedCreateOrUpdate", func() {
		var (
			ctx  context.Context
			ctrl *gomock.Controller
			c    *mockclient.MockClient
			s    *runtime.Scheme

			name      string
			namespace string
			obj       *unstructured.Unstructured
		)

		BeforeEach(func() {
			ctx = context.TODO()
			s = scheme.Scheme

			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			name = "foo"
			namespace = "bar"

			obj = &unstructured.Unstructured{}
			obj.SetName(name)
			obj.SetNamespace(namespace)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("kind registered in scheme (Deployment)", func() {
			var (
				deploymentGVK schema.GroupVersionKind

				currentDeployment             *appsv1.Deployment
				currentDeploymentUnstructured *unstructured.Unstructured
			)

			BeforeEach(func() {
				deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
				obj.SetGroupVersionKind(deploymentGVK)

				currentDeployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: pointer.Int32Ptr(1),
					},
				}

				currentDeploymentUnstructured = &unstructured.Unstructured{}
				Expect(s.Convert(currentDeployment, currentDeploymentUnstructured, nil)).To(Succeed(), "should be able to convert deployment to unstructured")
			})

			It("should make a typed get request and correctly create the object", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
						Return(apierrors.NewNotFound(appsv1.Resource("deployments"), name)),
					c.EXPECT().Create(ctx, obj),
				)

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, false, func() error {
					Expect(obj.Object["spec"]).To(BeNil(), "obj should not be filled, as the object does not exist yet")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should make a typed get request and skip update (no changes)", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, o runtime.Object) error {
						deploy, ok := o.(*appsv1.Deployment)
						Expect(ok).To(BeTrue())

						currentDeployment.DeepCopyInto(deploy)
						return nil
					})

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentDeploymentUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(operationType).To(Equal(controllerutil.OperationResultNone))
			})

			It("should make a typed get request and don't skip update (no changes but alwaysUpdate=false)", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, o runtime.Object) error {
							deploy, ok := o.(*appsv1.Deployment)
							Expect(ok).To(BeTrue())

							currentDeployment.DeepCopyInto(deploy)
							return nil
						}),
					c.EXPECT().Update(ctx, obj),
				)

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, true, func() error {
					Expect(obj).To(DeepEqual(currentDeploymentUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should make a typed get request and correctly update the object", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, o runtime.Object) error {
							deploy, ok := o.(*appsv1.Deployment)
							Expect(ok).To(BeTrue())

							currentDeployment.DeepCopyInto(deploy)
							return nil
						}),
					c.EXPECT().Update(ctx, obj),
				)

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentDeploymentUnstructured), "obj should be filled with the obj's current spec")

					// mutate object
					obj.SetLabels(map[string]string{
						"foo": "bar",
					})
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("kind registered in scheme (VerticalPodAutoscaler)", func() {
			var (
				vpaGVK schema.GroupVersionKind

				currentVPA             *autoscalerv1beta2.VerticalPodAutoscaler
				currentVPAUnstructured *unstructured.Unstructured
			)

			BeforeEach(func() {
				vpaGVK = autoscalerv1beta2.SchemeGroupVersion.WithKind("VerticalPodAutoscaler")
				obj.SetGroupVersionKind(vpaGVK)

				currentVPA = &autoscalerv1beta2.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: autoscalerv1beta2.VerticalPodAutoscalerSpec{
						TargetRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "kube-apiserver",
						},
					},
				}

				currentVPAUnstructured = &unstructured.Unstructured{}
				tmpScheme := runtime.NewScheme()
				Expect(autoscalerv1beta2.AddToScheme(tmpScheme)).To(Succeed(), "should be able to add autoscaler types to temporary scheme")
				Expect(tmpScheme.Convert(currentVPA, currentVPAUnstructured, nil)).To(Succeed(), "should be able to convert VPA to unstructured")
			})

			It("should fallback to an unstructured get request and correctly create the object", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&unstructured.Unstructured{})).
						Return(apierrors.NewNotFound(autoscalerv1beta2.Resource("verticalpodautoscalers"), name)),
					c.EXPECT().Create(ctx, obj),
				)

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, false, func() error {
					Expect(obj.Object["spec"]).To(BeNil(), "obj should not be filled, as the object does not exist yet")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fallback to an unstructured get request and skip update (no changes)", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&unstructured.Unstructured{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, o runtime.Object) error {
						vpa, ok := o.(*unstructured.Unstructured)
						Expect(ok).To(BeTrue())

						currentVPAUnstructured.DeepCopyInto(vpa)
						return nil
					})

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentVPAUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(operationType).To(Equal(controllerutil.OperationResultNone))
			})

			It("should fallback to an unstructured get request but don't skip update (no changes but alwaysUpdate=true)", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&unstructured.Unstructured{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, o runtime.Object) error {
							vpa, ok := o.(*unstructured.Unstructured)
							Expect(ok).To(BeTrue())

							currentVPAUnstructured.DeepCopyInto(vpa)
							return nil
						}),
					c.EXPECT().Update(ctx, obj),
				)

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, true, func() error {
					Expect(obj).To(DeepEqual(currentVPAUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fallback to an unstructured get request and correctly update the object", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gomock.AssignableToTypeOf(&unstructured.Unstructured{})).
						DoAndReturn(func(ctx context.Context, key client.ObjectKey, o runtime.Object) error {
							vpa, ok := o.(*unstructured.Unstructured)
							Expect(ok).To(BeTrue())

							currentVPAUnstructured.DeepCopyInto(vpa)
							return nil
						}),
					c.EXPECT().Update(ctx, obj),
				)

				operationType, err := TypedCreateOrUpdate(ctx, c, s, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentVPAUnstructured), "obj should be filled with the obj's current spec")

					// mutate object
					obj.SetLabels(map[string]string{
						"foo": "bar",
					})
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

var _ = Describe("#tryUpdate", func() {
	It("should set state to obj, when conflict occurs", func() {
		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		objInFakeClient := newInfraObj()
		objInFakeClient.Status.Conditions = []gardencorev1beta1.Condition{
			{Type: "Health", Reason: "reason", Message: "messages", Status: "status", LastUpdateTime: metav1.Now()},
		}

		c := fake.NewClientBuilder().WithScheme(s).WithObjects(objInFakeClient).Build()
		infraObj := newInfraObj()
		transform := func() error {
			infraState, _ := json.Marshal(state{"someState"})
			infraObj.GetExtensionStatus().SetState(&runtime.RawExtension{Raw: infraState})
			return nil
		}

		u := &conflictErrManager{
			conflictsBeforeUpdate: 2,
			client:                c,
		}

		tryUpdateErr := tryUpdate(context.TODO(), retry.DefaultRetry, c, infraObj, u.updateFunc, transform)
		Expect(tryUpdateErr).NotTo(HaveOccurred())

		objFromFakeClient := &extensionsv1alpha1.Infrastructure{}
		err := c.Get(context.TODO(), kutil.Key("infraNamespace", "infraName"), objFromFakeClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(objFromFakeClient).To(Equal(infraObj))
	})
})

type state struct {
	Name string `json:"name"`
}

func newInfraObj() *extensionsv1alpha1.Infrastructure {
	return &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infraName",
			Namespace: "infraNamespace",
		},
	}
}

type conflictErrManager struct {
	conflictsBeforeUpdate int
	conflictsOccured      int
	client                client.Client
}

func (c *conflictErrManager) updateFunc(ctx context.Context, obj client.Object, o ...client.UpdateOption) error {
	if c.conflictsBeforeUpdate == c.conflictsOccured {
		return c.client.Status().Update(ctx, obj, o...)
	}

	c.conflictsOccured++
	return apierrors.NewConflict(schema.GroupResource{}, "", nil)
}
