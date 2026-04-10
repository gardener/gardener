// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	. "github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("utils", func() {
	Describe("#TypedCreateOrUpdate", func() {
		var (
			ctx context.Context

			// fakeScheme is passed to the fake client — includes VPA so the client can store/retrieve VPA objects.
			fakeScheme *runtime.Scheme
			// scheme is passed to TypedCreateOrUpdate and may differ per context (e.g. VPA context excludes VPA
			// to exercise the unstructured fallback path).
			scheme *runtime.Scheme

			name      string
			namespace string
			obj       *unstructured.Unstructured
		)

		BeforeEach(func() {
			ctx = context.TODO()

			fakeScheme = runtime.NewScheme()
			Expect(k8sscheme.AddToScheme(fakeScheme)).To(Succeed())
			Expect(vpaautoscalingv1.AddToScheme(fakeScheme)).To(Succeed())

			scheme = fakeScheme

			name = "foo"
			namespace = "bar"

			obj = &unstructured.Unstructured{}
			obj.SetName(name)
			obj.SetNamespace(namespace)
		})

		Context("kind registered in scheme (Deployment)", func() {
			var (
				deploymentGVK                 schema.GroupVersionKind
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
						Replicas: ptr.To[int32](1),
					},
				}

				currentDeploymentUnstructured = &unstructured.Unstructured{}
				Expect(scheme.Convert(currentDeployment, currentDeploymentUnstructured, nil)).To(Succeed(), "should be able to convert deployment to unstructured")
			})

			It("should make a typed get request and correctly create the object", func() {
				typedGetCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
						if _, ok := o.(*appsv1.Deployment); ok {
							typedGetCalled = true
						}
						return cl.Get(ctx, key, o, opts...)
					},
				}).Build()

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, false, func() error {
					Expect(obj.Object["spec"]).To(BeNil(), "obj should not be filled, as the object does not exist yet")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
				Expect(typedGetCalled).To(BeTrue())
				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)).To(Succeed())
			})

			It("should make a typed get request and skip update (no changes)", func() {
				updateCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithObjects(currentDeployment.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, cl client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
						updateCalled = true
						return cl.Update(ctx, o, opts...)
					},
				}).Build()

				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, currentDeploymentUnstructured)).To(Succeed())

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentDeploymentUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(operationType).To(Equal(controllerutil.OperationResultNone))
				Expect(updateCalled).To(BeFalse())
			})

			It("should make a typed get request and don't skip update (no changes but alwaysUpdate=true)", func() {
				updateCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithObjects(currentDeployment.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, cl client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
						updateCalled = true
						return cl.Update(ctx, o, opts...)
					},
				}).Build()

				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, currentDeploymentUnstructured)).To(Succeed())

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, true, func() error {
					Expect(obj).To(DeepEqual(currentDeploymentUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(updateCalled).To(BeTrue())
			})

			It("should make a typed get request and correctly update the object", func() {
				updateCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithObjects(currentDeployment.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, cl client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
						updateCalled = true
						return cl.Update(ctx, o, opts...)
					},
				}).Build()

				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, currentDeploymentUnstructured)).To(Succeed())

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentDeploymentUnstructured), "obj should be filled with the obj's current spec")
					obj.SetLabels(map[string]string{"foo": "bar"})
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(updateCalled).To(BeTrue())

				updated := &appsv1.Deployment{}
				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, updated)).To(Succeed())
				Expect(updated.Labels).To(HaveKeyWithValue("foo", "bar"))
			})
		})

		Context("kind not registered in scheme (VerticalPodAutoscaler)", func() {
			var (
				vpaGVK                 schema.GroupVersionKind
				currentVPA             *vpaautoscalingv1.VerticalPodAutoscaler
				currentVPAUnstructured *unstructured.Unstructured
			)

			BeforeEach(func() {
				// scheme intentionally excludes VPA to exercise the unstructured fallback path.
				scheme = runtime.NewScheme()
				Expect(k8sscheme.AddToScheme(scheme)).To(Succeed())

				vpaGVK = vpaautoscalingv1.SchemeGroupVersion.WithKind("VerticalPodAutoscaler")
				obj.SetGroupVersionKind(vpaGVK)

				currentVPA = &vpaautoscalingv1.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
						TargetRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "kube-apiserver",
						},
					},
				}

				currentVPAUnstructured = &unstructured.Unstructured{}
				tmpScheme := runtime.NewScheme()
				Expect(vpaautoscalingv1.AddToScheme(tmpScheme)).To(Succeed(), "should be able to add autoscaler types to temporary scheme")
				Expect(tmpScheme.Convert(currentVPA, currentVPAUnstructured, nil)).To(Succeed(), "should be able to convert VPA to unstructured")
			})

			It("should fallback to an unstructured get request and correctly create the object", func() {
				unstructuredGetCalled := false
				createCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
						if _, ok := o.(*unstructured.Unstructured); ok {
							unstructuredGetCalled = true
						}
						return cl.Get(ctx, key, o, opts...)
					},
					Create: func(ctx context.Context, cl client.WithWatch, o client.Object, opts ...client.CreateOption) error {
						createCalled = true
						return cl.Create(ctx, o, opts...)
					},
				}).Build()

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, false, func() error {
					Expect(obj.Object["spec"]).To(BeNil(), "obj should not be filled, as the object does not exist yet")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
				Expect(unstructuredGetCalled).To(BeTrue())
				Expect(createCalled).To(BeTrue())
			})

			It("should fallback to an unstructured get request and skip update (no changes)", func() {
				updateCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithObjects(currentVPA.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, cl client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
						updateCalled = true
						return cl.Update(ctx, o, opts...)
					},
				}).Build()

				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, currentVPAUnstructured)).To(Succeed())

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentVPAUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(operationType).To(Equal(controllerutil.OperationResultNone))
				Expect(updateCalled).To(BeFalse())
			})

			It("should fallback to an unstructured get request but don't skip update (no changes but alwaysUpdate=true)", func() {
				updateCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithObjects(currentVPA.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, cl client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
						updateCalled = true
						return cl.Update(ctx, o, opts...)
					},
				}).Build()

				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, currentVPAUnstructured)).To(Succeed())

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, true, func() error {
					Expect(obj).To(DeepEqual(currentVPAUnstructured), "obj should be filled with the obj's current spec")
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(updateCalled).To(BeTrue())
			})

			It("should fallback to an unstructured get request and correctly update the object", func() {
				updateCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(fakeScheme).WithObjects(currentVPA.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, cl client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
						updateCalled = true
						return cl.Update(ctx, o, opts...)
					},
				}).Build()

				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, currentVPAUnstructured)).To(Succeed())

				operationType, err := TypedCreateOrUpdate(ctx, c, scheme, obj, false, func() error {
					Expect(obj).To(DeepEqual(currentVPAUnstructured), "obj should be filled with the obj's current spec")
					obj.SetLabels(map[string]string{"foo": "bar"})
					return nil
				})

				Expect(operationType).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(updateCalled).To(BeTrue())

				updated := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, updated)).To(Succeed())
				Expect(updated.Labels).To(HaveKeyWithValue("foo", "bar"))
			})
		})
	})
})
