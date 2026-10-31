// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var _ = Describe("Controller", func() {
	Describe("#injectLabels", func() {
		var (
			obj, expected *unstructured.Unstructured
			labels        map[string]string
		)

		BeforeEach(func() {
			obj = &unstructured.Unstructured{Object: map[string]any{}}
			expected = obj.DeepCopy()
		})

		It("do nothing as labels is nil", func() {
			labels = nil
			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("do nothing as labels is empty", func() {
			labels = map[string]string{}
			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("should correctly inject labels into the object's metadata", func() {
			labels = map[string]string{
				"inject": "me",
			}
			expected.SetLabels(labels)

			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("should correctly inject labels into the object's pod template's metadata", func() {
			labels = map[string]string{
				"inject": "me",
			}

			// add .spec.template to object
			Expect(unstructured.SetNestedMap(obj.Object, map[string]any{
				"template": map[string]any{},
			}, "spec")).To(Succeed())

			expected = obj.DeepCopy()
			Expect(unstructured.SetNestedMap(expected.Object, map[string]any{
				"inject": "me",
			}, "spec", "template", "metadata", "labels")).To(Succeed())
			expected.SetLabels(labels)

			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("should correctly inject labels into the object's volumeClaimTemplates' metadata", func() {
			labels = map[string]string{
				"inject": "me",
			}

			// add .spec.volumeClaimTemplates to object
			Expect(unstructured.SetNestedMap(obj.Object, map[string]any{
				"volumeClaimTemplates": []any{
					map[string]any{
						"metadata": map[string]any{
							"name": "volume-claim-name",
						},
					},
				},
			}, "spec")).To(Succeed())

			expected = obj.DeepCopy()
			Expect(unstructured.SetNestedMap(expected.Object, map[string]any{
				"volumeClaimTemplates": []any{
					map[string]any{
						"metadata": map[string]any{
							"name": "volume-claim-name",
							"labels": map[string]any{
								"inject": "me",
							},
						},
					},
				},
			}, "spec")).To(Succeed())
			expected.SetLabels(labels)

			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})
	})

	Describe("#deletionPropagation", func() {
		var obj *unstructured.Unstructured

		BeforeEach(func() {
			obj = &unstructured.Unstructured{Object: map[string]any{}}
		})

		It("should return an empty string when the deletion propagation is not set", func() {
			Expect(deletionPropagation(obj)).To(BeEmpty())
		})

		It("should return an error when the deletion propagation is invalid", func() {
			Expect(unstructured.SetNestedField(obj.Object, "invalid", "metadata", "annotations", resourcesv1alpha1.DeletionPropagationOnInvalidUpdate)).To(Succeed())

			Expect(deletionPropagation(obj)).Error().To(HaveOccurred())
		})

		It("should return the correct deletion propagation value when it is set", func() {
			Expect(unstructured.SetNestedField(obj.Object, string(metav1.DeletePropagationOrphan), "metadata", "annotations", resourcesv1alpha1.DeletionPropagationOnInvalidUpdate)).To(Succeed())

			Expect(deletionPropagation(obj)).To(Equal(metav1.DeletePropagationOrphan))
		})
	})

	Describe("#forceDeletion", func() {
		It("should return true when the force-deletion annotation is set to true", func() {
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{resourcesv1alpha1.ForceDeletion: "true"},
				},
			}
			Expect(forceDeletion(mr)).To(BeTrue())
		})

		It("should return false when the force-deletion annotation is not set", func() {
			mr := &resourcesv1alpha1.ManagedResource{}
			Expect(forceDeletion(mr)).To(BeFalse())
		})

		It("should return false when the force-deletion annotation is set to false", func() {
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{resourcesv1alpha1.ForceDeletion: "false"},
				},
			}
			Expect(forceDeletion(mr)).To(BeFalse())
		})
	})

	Describe("#clearNamespaceSpecFinalizers", func() {
		var (
			ctx           context.Context
			obj           *unstructured.Unstructured
			namespaceName string
		)

		BeforeEach(func() {
			ctx = context.TODO()
			namespaceName = "test-namespace"
			obj = &unstructured.Unstructured{}
			obj.SetAPIVersion("v1")
			obj.SetKind("Namespace")
			obj.SetName(namespaceName)
		})

		It("should clear spec.finalizers via the /finalize subresource", func() {
			subResourceUpdateCalled := false
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					SubResourceUpdate: func(_ context.Context, _ client.Client, subResourceName string, o client.Object, _ ...client.SubResourceUpdateOption) error {
						subResourceUpdateCalled = true
						Expect(subResourceName).To(Equal("finalize"))
						ns, ok := o.(*corev1.Namespace)
						Expect(ok).To(BeTrue())
						Expect(ns.Spec.Finalizers).To(BeEmpty())
						return nil
					},
				}).Build()

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
				Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{"kubernetes"}},
			}
			Expect(fakeClient.Create(ctx, ns)).To(Succeed())

			Expect(clearNamespaceSpecFinalizers(ctx, fakeClient, obj)).To(Succeed())
			Expect(subResourceUpdateCalled).To(BeTrue())
		})

		It("should not call /finalize subresource if spec.finalizers is empty", func() {
			subResourceUpdateCalled := false
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ ...client.SubResourceUpdateOption) error {
						subResourceUpdateCalled = true
						return nil
					},
				}).Build()

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			Expect(fakeClient.Create(ctx, ns)).To(Succeed())

			Expect(clearNamespaceSpecFinalizers(ctx, fakeClient, obj)).To(Succeed())
			Expect(subResourceUpdateCalled).To(BeFalse())
		})

		It("should return nil if the namespace does not exist", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

			Expect(clearNamespaceSpecFinalizers(ctx, fakeClient, obj)).To(Succeed())
		})

		It("should return an error if the Get call fails", func() {
			fakeErr := errors.New("get error")
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fakeErr
					},
				}).Build()

			err := clearNamespaceSpecFinalizers(ctx, fakeClient, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("get error"))
		})
	})
})
