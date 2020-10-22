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

package webhooks_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/gardener/gardener/pkg/admissioncontroller/server/handlers/webhooks"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ValidateNamespaceDeletion", func() {
	var (
		ctx       = context.Background()
		namespace = func() *corev1.Namespace {
			return &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}
		}
	)

	DescribeTable("Namespace deletion admission",
		func(namespace func() *corev1.Namespace, mock *mockClient, op admissionv1beta1.Operation, expectedAllowed bool, expectedMsg string) {
			defer mock.ctrl.Finish()
			cache := mockcache.NewMockCache(mock.ctrl)

			coreInformerFactory := coreinformers.NewSharedInformerFactory(nil, 0)
			projectInformer := newSyncedInformer(coreInformerFactory.Core().V1beta1().Projects().Informer())
			cache.EXPECT().GetInformer(gomock.Any(), &gardencorev1beta1.Project{}).Return(projectInformer, nil)
			shootInformer := newSyncedInformer(coreInformerFactory.Core().V1beta1().Shoots().Informer())
			cache.EXPECT().GetInformer(gomock.Any(), &gardencorev1beta1.Shoot{}).Return(shootInformer, nil)

			clientSet := fake.NewClientSetBuilder().WithClient(mock).WithDirectClient(mock).WithCache(cache).Build()
			validationHandler, err := webhooks.NewValidateNamespaceDeletionHandler(ctx, clientSet)
			Expect(err).ToNot(HaveOccurred())

			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			user := authenticationv1.UserInfo{
				Username: "user",
				Groups:   []string{"group"},
			}

			request := createHTTPRequest(namespace(), scheme, user, op)
			response := httptest.NewRecorder()

			validationHandler.ServeHTTP(response, request)

			admissionReview := &admissionv1beta1.AdmissionReview{}
			Expect(decodeAdmissionResponse(response, admissionReview)).To(Succeed())
			Expect(response).Should(HaveHTTPStatus(http.StatusOK))
			Expect(admissionReview.Response).To(Not(BeNil()))
			Expect(admissionReview.Response.Allowed).To(Equal(expectedAllowed))
			if expectedMsg != "" {
				Expect(admissionReview.Response.Result).To(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Message": ContainSubstring(expectedMsg),
					})))
			}

		},
		Entry("should pass because no project or shoots available", namespace,
			newMockClient().withProjects(),
			admissionv1beta1.Delete, true, emptyMessage),
		Entry("should fail because of empty request", func() *corev1.Namespace { return nil },
			newMockClient(), admissionv1beta1.Delete, false, "invalid request body (missing admission request)"),
		Entry("should pass because of update request", namespace,
			newMockClient(), admissionv1beta1.Update, true, emptyMessage),
		Entry("should pass because namespace is not project related", namespace,
			newMockClient().
				withProjects(
					project("namespace1", "project1"),
					project("namespace2", "project2"),
				), admissionv1beta1.Delete, true, emptyMessage),
		Entry("should pass because namespace is used by project w/ deletionTimestamp", namespace,
			newMockClient().
				withProjects(
					project("namespace1", "project1"),
					func() gardencorev1beta1.Project {
						p := project(namespace().Name, "project2")
						t := metav1.Now()
						p.SetDeletionTimestamp(&t)
						return p
					}(),
				).
				withNamespace(namespace()).
				withShoots(client.InNamespace(namespace().Name)),
			admissionv1beta1.Delete, true, emptyMessage),
		Entry("should not pass because namespace is used by project w/ deletionTimestamp but has shoots", namespace,
			newMockClient().
				withProjects(
					project("namespace1", "project1"),
					func() gardencorev1beta1.Project {
						p := project(namespace().Name, "project2")
						t := metav1.Now()
						p.SetDeletionTimestamp(&t)
						return p
					}(),
				).
				withNamespace(namespace()).
				withShoots(client.InNamespace(namespace().Name), shoot("namespace1", "shoot1")),
			admissionv1beta1.Delete, false, "there are still Shoots"),
		Entry("should not pass because namespace is used by project", namespace,
			newMockClient().
				withProjects(
					project("namespace1", "project1"),
					project(namespace().Name, "project2"),
				).
				withNamespace(namespace()),
			admissionv1beta1.Delete, false, "you must delete the corresponding project \"project2\""),
		Entry("should pass because namespace is used by project but not found by client", namespace,
			func() client.Client {
				cl := newMockClient().
					withProjects(
						project("namespace1", "project1"),
						project(namespace().Name, "project2"),
					)
				cl.EXPECT().Get(gomock.Any(), kutil.Key(namespace().Name), &corev1.Namespace{}).Return(apierrors.NewNotFound(corev1.Resource("namespace"), namespace().Name))
				return cl
			}(),
			admissionv1beta1.Delete, true, emptyMessage),
		Entry("should pass because namespace already has a deletionTimestamp", namespace,
			newMockClient().
				withProjects(
					project("namespace1", "project1"),
					project(namespace().Name, "project2"),
				).
				withNamespace(func() *corev1.Namespace {
					n := namespace()
					t := metav1.Now()
					n.SetDeletionTimestamp(&t)
					return n
				}()),
			admissionv1beta1.Delete, true, emptyMessage),
		Entry("should not pass because namespace cannot be fetched", namespace,
			func() client.Client {
				cl := newMockClient().withProjects(
					project("namespace1", "project1"),
					project(namespace().Name, "project2"),
				)
				cl.EXPECT().Get(gomock.Any(), kutil.Key(namespace().Name), &corev1.Namespace{}).Return(errors.New("foo"))
				return cl
			}(),
			admissionv1beta1.Delete, false, "foo"),
		Entry("should not pass because projects cannot be fetched", namespace,
			func() client.Client {
				cl := newMockClient()
				cl.EXPECT().List(gomock.Any(), &gardencorev1beta1.ProjectList{}).Return(errors.New("foo"))
				return cl
			}(),
			admissionv1beta1.Delete, false, "foo"),
	)
})

type mockClient struct {
	*mockclient.MockClient
	ctrl *gomock.Controller
}

func newMockClient() *mockClient {
	ctrl := gomock.NewController(GinkgoT())
	return &mockClient{ctrl: ctrl, MockClient: mockclient.NewMockClient(ctrl)}
}

func (c *mockClient) withShoots(listOption client.ListOption, shoots ...gardencorev1beta1.Shoot) *mockClient {
	c.EXPECT().List(gomock.Any(), &gardencorev1beta1.ShootList{}, listOption).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
		list.Items = shoots
		return nil
	})
	return c
}

func (c *mockClient) withProjects(projects ...gardencorev1beta1.Project) *mockClient {
	c.EXPECT().List(gomock.Any(), &gardencorev1beta1.ProjectList{}).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ProjectList, _ ...client.ListOption) error {
		list.Items = projects
		return nil
	})
	return c
}

func (c *mockClient) withNamespace(namespace *corev1.Namespace) *mockClient {
	c.EXPECT().Get(gomock.Any(), kutil.Key(namespace.Name), &corev1.Namespace{}).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace) error {
		*obj = *namespace
		return nil
	})
	return c
}

func project(namespace, name string) gardencorev1beta1.Project {
	namespaceRef := namespace
	return gardencorev1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gardencorev1beta1.ProjectSpec{
			Namespace: &namespaceRef,
		},
	}
}

func shoot(namespace, name string) gardencorev1beta1.Shoot {
	return gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newSyncedInformer(informer cache.SharedIndexInformer) cache.SharedIndexInformer {
	return &syncedInformer{informer}
}

// syncedInformer is an SharedIndexInformer that is always synced and thus doesn't need to be started against a running API.
type syncedInformer struct {
	cache.SharedIndexInformer
}

func (i *syncedInformer) HasSynced() bool {
	return true
}
