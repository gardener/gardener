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

package seedadmission_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenlogger "github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/seedadmission"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Extension CRDs", func() {
	Describe("#ValidateExtensionDeletion", func() {
		var (
			ctx     = context.Background()
			logger  = gardenlogger.NewNopLogger()
			request admission.Request
			decoder *admission.Decoder

			ctrl *gomock.Controller
			c    *mockclient.MockClient

			validator admission.Handler
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			request = admission.Request{}
			request.Operation = admissionv1.Delete

			var err error
			decoder, err = admission.NewDecoder(kubernetes.SeedScheme)
			Expect(err).NotTo(HaveOccurred())

			validator = NewExtensionDeletionProtection(logger)
			Expect(inject.ClientInto(c, validator)).To(BeTrue())
			Expect(admission.InjectDecoderInto(decoder, validator)).To(BeTrue())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		var (
			resources = []metav1.GroupVersionResource{
				{Group: apiextensionsv1beta1.SchemeGroupVersion.Group, Version: apiextensionsv1beta1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"},
				{Group: apiextensionsv1.SchemeGroupVersion.Group, Version: apiextensionsv1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"},

				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupbuckets"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupentries"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "containerruntimes"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "controlplanes"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "extensions"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "infrastructures"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "networks"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "operatingsystemconfigs"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "workers"},
			}
			crdResources   = resources[0:2]
			otherResources = resources[2:]
			fooResource    = metav1.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"}

			deletionUnprotectedLabels    = map[string]string{common.GardenerDeletionProtected: "false"}
			deletionProtectedLabels      = map[string]string{common.GardenerDeletionProtected: "true"}
			deletionConfirmedAnnotations = map[string]string{common.ConfirmationDeletion: "true"}
		)

		testDeletionUnconfirmed := func(ctx context.Context, request admission.Request, resource metav1.GroupVersionResource) {
			request.Resource = resource
			expectDenied(validator.Handle(ctx, request), ContainSubstring("annotation to delete"), resourceToId(resource))
		}

		testDeletionConfirmed := func(ctx context.Context, request admission.Request, resource metav1.GroupVersionResource) {
			request.Resource = resource
			expectAllowed(validator.Handle(ctx, request), Equal(""), resourceToId(resource))
		}

		Context("ignored requests", func() {
			It("should ignore other operations than DELETE", func() {
				request.Operation = admissionv1.Create
				expectAllowed(validator.Handle(ctx, request), ContainSubstring("not DELETE"))
				request.Operation = admissionv1.Update
				expectAllowed(validator.Handle(ctx, request), ContainSubstring("not DELETE"))
				request.Operation = admissionv1.Connect
				expectAllowed(validator.Handle(ctx, request), ContainSubstring("not DELETE"))
			})

			It("should ignore types other than CRDs + extension resources", func() {
				request.Resource = fooResource
				expectAllowed(validator.Handle(ctx, request), ContainSubstring("resource is not deletion-protected"))
			})
		})

		Context("old object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
			})

			It("should return an error because the old object cannot be decoded", func() {
				for _, resource := range resources {
					request.Resource = resource
					request.OldObject = runtime.RawExtension{Raw: []byte("foo")}
					expectErrored(validator.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), ContainSubstring("invalid character"), resourceToId(resource))
				}
			})

			Context("custom resource definitions", func() {
				BeforeEach(func() {
					request.Kind = metav1.GroupVersionKind{Kind: "CustomResourceDefinition"}
				})

				It("should admit the deletion because CRD has no protection label", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionUnprotectedLabels, nil)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, nil)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})

			Context("other resources", func() {
				It("should prevent the deletion because deletion is not confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, deletionConfirmedAnnotations)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})
		})

		Context("new object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
			})

			It("should return an error because the new object cannot be decoded", func() {
				for _, resource := range resources {
					request.Resource = resource
					request.Object = runtime.RawExtension{Raw: []byte("foo")}

					expectErrored(validator.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), ContainSubstring("invalid character"), resourceToId(resource))
				}
			})

			Context("custom resource definitions", func() {
				BeforeEach(func() {
					request.Kind = metav1.GroupVersionKind{Kind: "CustomResourceDefinition"}
				})

				It("should admit the deletion because CRD has no protection label", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionUnprotectedLabels, nil)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, nil)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})

			Context("other resources", func() {
				It("should prevent the deletion because deletion is not confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, deletionConfirmedAnnotations)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})
		})

		Context("object must be looked up (DELETE)", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
			})

			It("should return an error because the GET call failed", func() {
				for _, resource := range resources {
					fakeErr := errors.New("fake")
					prepareRequestAndObjectWithResource(&request, obj, resource)
					request.Resource = resource
					request.Name = "foo"

					c.EXPECT().Get(gomock.Any(), gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&unstructured.Unstructured{})).Return(fakeErr)

					expectErrored(validator.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), Equal(fakeErr.Error()), resourceToId(resource))
				}
			})

			It("should return no error because the GET call returned 'not found'", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(&request, obj, resource)
					request.Resource = resource
					request.Name = "foo"

					c.EXPECT().Get(gomock.Any(), gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&unstructured.Unstructured{})).Return(apierrors.NewNotFound(core.Resource(resource.Resource), "name"))

					expectAllowed(validator.Handle(ctx, request), ContainSubstring("object was not found"), resourceToId(resource))
				}
			})

			Context("custom resource definitions", func() {
				BeforeEach(func() {
					request.Kind = metav1.GroupVersionKind{Kind: "CustomResourceDefinition"}
					request.Name = "foo-crd"
				})

				It("should admit the deletion because CRD has no protection label", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)

						c.EXPECT().Get(gomock.Any(), kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
							prepareObjectWithLabelsAnnotations(obj, resource, nil, nil)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)

						c.EXPECT().Get(gomock.Any(), kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
							prepareObjectWithLabelsAnnotations(obj, resource, deletionUnprotectedLabels, nil)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)

						c.EXPECT().Get(gomock.Any(), kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
							prepareObjectWithLabelsAnnotations(obj, resource, deletionProtectedLabels, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)

						c.EXPECT().Get(gomock.Any(), kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
							prepareObjectWithLabelsAnnotations(obj, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})

			Context("other resources", func() {
				BeforeEach(func() {
					request.Name = "foo"
					request.Namespace = "bar"
				})

				It("should prevent the deletion because deletion is not confirmed", func() {
					for _, resource := range otherResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)

						c.EXPECT().Get(gomock.Any(), kutil.Key(request.Namespace, request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
							prepareObjectWithLabelsAnnotations(obj, resource, nil, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)

						c.EXPECT().Get(gomock.Any(), kutil.Key(request.Namespace, request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
							prepareObjectWithLabelsAnnotations(obj, resource, nil, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})
		})

		Context("object must be looked up (DELETE COLLECTION)", func() {
			var obj *unstructured.UnstructuredList

			BeforeEach(func() {
				obj = &unstructured.UnstructuredList{}
				obj.SetKind("List")
			})

			It("should return an error because the LIST call failed", func() {
				for _, resource := range resources {
					fakeErr := errors.New("fake")
					prepareRequestAndObjectWithResource(&request, obj, resource)
					request.Resource = resource
					obj.SetKind("List")

					c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).Return(fakeErr)

					expectErrored(validator.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), Equal(fakeErr.Error()), resourceToId(resource))
				}
			})

			It("should return no error because the LIST call returned 'not found'", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(&request, obj, resource)
					request.Resource = resource
					obj.SetKind("List")

					c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).Return(apierrors.NewNotFound(core.Resource(resource.Resource), "name"))

					expectAllowed(validator.Handle(ctx, request), ContainSubstring("object was not found"), resourceToId(resource))
				}
			})

			Context("custom resource definitions", func() {
				BeforeEach(func() {
					request.Kind = metav1.GroupVersionKind{Kind: "CustomResourceDefinition"}
				})

				It("should admit the deletion because CRD has no protection label", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, nil, nil)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, deletionUnprotectedLabels, nil)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, deletionProtectedLabels, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})

			Context("other resources", func() {
				BeforeEach(func() {
					request.Namespace = "bar"
				})

				It("should prevent the deletion because deletion is not confirmed", func() {
					for _, resource := range otherResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, nil, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						prepareRequestAndObjectWithResource(&request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(gomock.Any(), obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, nil, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, request, resource)
					}
				})
			})
		})
	})
})

func resourceToId(resource metav1.GroupVersionResource) string {
	return fmt.Sprintf("%s/%s/%s", resource.Group, resource.Version, resource.Resource)
}

type unstructuredInterface interface {
	SetAPIVersion(string)
	SetKind(string)
}

func prepareRequestAndObjectWithResource(request *admission.Request, obj unstructuredInterface, resource metav1.GroupVersionResource) {
	request.Kind.Group = resource.Group
	request.Kind.Version = resource.Version
	obj.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
	obj.SetKind(request.Kind.Kind)
}

func prepareObjectWithLabelsAnnotations(obj runtime.Object, resource metav1.GroupVersionResource, labels, annotations map[string]string) {
	switch obj := obj.(type) {
	case *unstructured.Unstructured:
		obj.SetAPIVersion(fmt.Sprintf("%s/%s", resource.Group, resource.Version))
		obj.SetKind(resource.Resource)
		obj.SetLabels(labels)
		obj.SetAnnotations(annotations)
	case *unstructured.UnstructuredList:
		o := &unstructured.Unstructured{}
		o.SetAPIVersion(fmt.Sprintf("%s/%s", resource.Group, resource.Version))
		o.SetKind(resource.Resource)
		o.SetLabels(labels)
		o.SetAnnotations(annotations)
		obj.Items = []unstructured.Unstructured{*o}
	}
}

func getObjectJSONWithLabelsAnnotations(obj *unstructured.Unstructured, resource metav1.GroupVersionResource, labels, annotations map[string]string) []byte {
	prepareObjectWithLabelsAnnotations(obj, resource, labels, annotations)

	objJSON, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())

	return objJSON
}
