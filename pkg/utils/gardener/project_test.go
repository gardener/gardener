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

package gardener_test

import (
	"context"
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinternallisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		projectInternal = &gardencore.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencore.ProjectSpec{
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

	Describe("#ProjectForNamespaceFromInternalLister", func() {
		var lister *fakeInternalLister

		BeforeEach(func() {
			lister = &fakeInternalLister{}
		})

		It("should return an error because listing failed", func() {
			lister.err = fakeErr

			result, err := ProjectForNamespaceFromInternalLister(lister, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(result).To(BeNil())
		})

		It("should return the found project", func() {
			lister.projects = []*gardencore.Project{projectInternal}

			result, err := ProjectForNamespaceFromInternalLister(lister, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(projectInternal))
		})

		It("should return a 'not found' error", func() {
			result, err := ProjectForNamespaceFromInternalLister(lister, namespaceName)
			Expect(err).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: "core.gardener.cloud", Resource: "Project"}, namespaceName)))
			Expect(result).To(BeNil())
		})
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
			c.EXPECT().Get(ctx, kutil.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(fakeErr)

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult).To(BeNil())
			Expect(projectResult).To(BeNil())
		})

		It("should return the namespace but no project because labels missing", func() {
			c.EXPECT().Get(ctx, kutil.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace) error {
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

			c.EXPECT().Get(ctx, kutil.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace) error {
				namespace.DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().Get(ctx, kutil.Key(projectName), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).Return(fakeErr)

			projectResult, namespaceResult, err := ProjectAndNamespaceFromReader(ctx, c, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(namespaceResult).To(Equal(namespace))
			Expect(projectResult).To(BeNil())
		})

		It("should return both namespace and project", func() {
			namespace.Labels = map[string]string{"project.gardener.cloud/name": projectName}

			c.EXPECT().Get(ctx, kutil.Key(namespaceName), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace) error {
				namespace.DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().Get(ctx, kutil.Key(projectName), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *gardencorev1beta1.Project) error {
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

type fakeLister struct {
	gardencorelisters.ProjectLister
	projects []*gardencorev1beta1.Project
	err      error
}

func (c *fakeLister) List(labels.Selector) ([]*gardencorev1beta1.Project, error) {
	return c.projects, c.err
}

type fakeInternalLister struct {
	gardencoreinternallisters.ProjectLister
	projects []*gardencore.Project
	err      error
}

func (c *fakeInternalLister) List(labels.Selector) ([]*gardencore.Project, error) {
	return c.projects, c.err
}
