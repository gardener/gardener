// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Project", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake err")

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
			Expect(err).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: gardencorev1beta1.GroupName, Resource: "Project"}, "<unknown>")))
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
			c.EXPECT().Get(ctx, kubernetesutils.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(fakeErr)

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult).To(BeNil())
			Expect(projectResult).To(BeNil())
		})

		It("should return the namespace but no project because labels missing", func() {
			c.EXPECT().Get(ctx, kubernetesutils.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
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

			c.EXPECT().Get(ctx, kubernetesutils.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				namespace.DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().Get(ctx, kubernetesutils.Key(projectName), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).Return(fakeErr)

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult).To(Equal(namespace))
			Expect(projectResult).To(BeNil())
		})

		It("should return both namespace and project", func() {
			namespace.Labels = map[string]string{"project.gardener.cloud/name": projectName}

			c.EXPECT().Get(ctx, kubernetesutils.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				namespace.DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().Get(ctx, kubernetesutils.Key(projectName), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *gardencorev1beta1.Project, _ ...client.GetOption) error {
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
