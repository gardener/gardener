// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Project", func() {
	var (
		fakeClient client.Client

		ctx     = context.TODO()
		fakeErr = errors.New("fake err")

		namespaceName = "foo"

		projectName = "bar"
		project     *gardencorev1beta1.Project
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.Project{}, gardencore.ProjectNamespace, indexer.ProjectNamespaceIndexerFunc).
			Build()

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
	})

	Describe("#ProjectForNamespaceFromReader", func() {
		It("should return an error because the listing failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return fakeErr
					},
				}).
				Build()

			projectResult, err := ProjectForNamespaceFromReader(ctx, fakeClient, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(projectResult).To(BeNil())
		})

		It("should return an error because the listing yielded no results", func() {
			projectResult, err := ProjectForNamespaceFromReader(ctx, fakeClient, namespaceName)
			Expect(err).To(BeNotFoundError())
			Expect(projectResult).To(BeNil())
		})

		It("should return the project", func() {
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			projectResult, err := ProjectForNamespaceFromReader(ctx, fakeClient, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(projectResult.Name).To(Equal(projectName))
			Expect(projectResult.Spec.Namespace).To(Equal(&namespaceName))
		})
	})

	Describe("#ProjectAndNamespaceFromReader", func() {
		It("should return an error because getting the namespace failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fakeErr
					},
				}).
				Build()

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, fakeClient, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult).To(BeNil())
			Expect(projectResult).To(BeNil())
		})

		It("should return the namespace but no project because labels missing", func() {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, fakeClient, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(namespaceResult.Name).To(Equal(namespaceName))
			Expect(projectResult).To(BeNil())
		})

		It("should return an error because getting the project failed", func() {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   namespaceName,
					Labels: map[string]string{"project.gardener.cloud/name": projectName},
				},
			}
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithObjects(namespace).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*gardencorev1beta1.Project); ok {
							return fakeErr
						}
						return cl.Get(ctx, key, obj, opts...)
					},
				}).
				Build()

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, fakeClient, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult.Name).To(Equal(namespaceName))
			Expect(projectResult).To(BeNil())
		})

		It("should return both namespace and project", func() {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   namespaceName,
					Labels: map[string]string{"project.gardener.cloud/name": projectName},
				},
			}
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, fakeClient, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(namespaceResult.Name).To(Equal(namespaceName))
			Expect(projectResult.Name).To(Equal(projectName))
		})
	})
})
