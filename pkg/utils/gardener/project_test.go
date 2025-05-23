// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Project", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = errors.New("fake err")

		namespaceName = "foo"
		namespace     = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}

		projectName = "bar"
		project     = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ProjectForNamespaceFromReader", func() {
		It("should return an error because the listing failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}), client.MatchingFields{gardencore.ProjectNamespace: namespaceName}).Return(fakeErr)

			projectResult, err := ProjectForNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(projectResult).To(BeNil())
		})

		It("should return an error because the listing yielded no results", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}), client.MatchingFields{gardencore.ProjectNamespace: namespaceName})

			projectResult, err := ProjectForNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(BeNotFoundError())
			Expect(projectResult).To(BeNil())
		})

		It("should return the project", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}), client.MatchingFields{gardencore.ProjectNamespace: namespaceName}).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ProjectList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ProjectList{Items: []gardencorev1beta1.Project{*project}}).DeepCopyInto(list)
				return nil
			})

			projectResult, err := ProjectForNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(projectResult).To(Equal(project))
		})
	})

	Describe("#ProjectAndNamespaceFromReader", func() {
		It("should return an error because getting the namespace failed", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(fakeErr)

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult).To(BeNil())
			Expect(projectResult).To(BeNil())
		})

		It("should return the namespace but no project because labels missing", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				namespace.DeepCopyInto(obj)
				return nil
			})

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(namespaceResult).To(Equal(namespace))
			Expect(projectResult).To(BeNil())
		})

		It("should return an error because getting the project failed", func() {
			namespace.Labels = map[string]string{"project.gardener.cloud/name": projectName}

			c.EXPECT().Get(ctx, client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				namespace.DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().Get(ctx, client.ObjectKey{Name: projectName}, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).Return(fakeErr)

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult).To(Equal(namespace))
			Expect(projectResult).To(BeNil())
		})

		It("should return both namespace and project", func() {
			namespace.Labels = map[string]string{"project.gardener.cloud/name": projectName}

			c.EXPECT().Get(ctx, client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				namespace.DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().Get(ctx, client.ObjectKey{Name: projectName}, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Project, _ ...client.GetOption) error {
				project.DeepCopyInto(obj)
				return nil
			})

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(namespaceResult).To(Equal(namespace))
			Expect(projectResult).To(Equal(project))
		})
	})
})
