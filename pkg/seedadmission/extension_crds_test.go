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
	"path/filepath"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/seedadmission"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Extension Deletion Protection", func() {
	Describe("#ValidateExtensionDeletion", func() {
		var (
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

			validator = &ExtensionDeletionProtection{}
			Expect(inject.LoggerInto(logger, validator)).To(BeTrue())
			Expect(inject.APIReaderInto(c, validator)).To(BeTrue())
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

		resourceToId := func(resource metav1.GroupVersionResource) string {
			return fmt.Sprintf("%s/%s/%s", resource.Group, resource.Version, resource.Resource)
		}

		testDeletionUnconfirmed := func(ctx context.Context, request admission.Request, resource metav1.GroupVersionResource) {
			request.Resource = resource
			expectDenied(validator.Handle(ctx, request), ContainSubstring("annotation to delete"), resourceToId(resource))
		}

		testDeletionConfirmed := func(ctx context.Context, request admission.Request, resource metav1.GroupVersionResource) {
			request.Resource = resource
			expectAllowed(validator.Handle(ctx, request), Equal(""), resourceToId(resource))
		}

		type unstructuredInterface interface {
			SetAPIVersion(string)
			SetKind(string)
		}

		prepareRequestAndObjectWithResource := func(request *admission.Request, obj unstructuredInterface, resource metav1.GroupVersionResource) {
			request.Kind.Group = resource.Group
			request.Kind.Version = resource.Version
			obj.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
			obj.SetKind(request.Kind.Kind)
		}

		prepareObjectWithLabelsAnnotations := func(obj runtime.Object, resource metav1.GroupVersionResource, labels, annotations map[string]string) {
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

		getObjectJSONWithLabelsAnnotations := func(obj *unstructured.Unstructured, resource metav1.GroupVersionResource, labels, annotations map[string]string) []byte {
			prepareObjectWithLabelsAnnotations(obj, resource, labels, annotations)

			objJSON, err := json.Marshal(obj)
			Expect(err).NotTo(HaveOccurred())

			return objJSON
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

	Describe("Integration Test", func() {
		var (
			c         client.Client
			namespace = "shoot--foo--bar"

			objects = []client.Object{
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupbuckets.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupentries.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "containerruntimes.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "controlplanes.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "extensions.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "infrastructures.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "networks.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "operatingsystemconfigs.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "workers.extensions.gardener.cloud"}},
				&extensionsv1alpha1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
				&extensionsv1alpha1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
				&extensionsv1alpha1.ContainerRuntime{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.ControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Network{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			}
			crdObjects       = objects[0:9]
			extensionObjects = objects[9:]
			podObject        = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "foo",
						Image: "foo:latest",
					}},
				},
			}

			deletionUnprotectedLabels    = map[string]string{common.GardenerDeletionProtected: "false"}
			deletionConfirmedAnnotations = map[string]string{common.ConfirmationDeletion: "true"}
		)

		BeforeEach(func() {
			c, err = client.New(restConfig, client.Options{Scheme: kubernetes.SeedScheme})
			Expect(err).NotTo(HaveOccurred())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			if err := c.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("applying CRDs")
			applier, err := kubernetes.NewChartApplierForConfig(restConfig)
			Expect(err).NotTo(HaveOccurred())
			repoRoot := filepath.Join("..", "..")
			Expect(applier.Apply(ctx, filepath.Join(repoRoot, "charts", "seed-bootstrap", "charts", "extensions"), "extensions", "")).To(Succeed())

			Eventually(func() bool {
				for _, object := range extensionObjects {
					err := c.Get(ctx, client.ObjectKeyFromObject(object), object)
					if meta.IsNoMatchError(err) {
						return false
					}
				}
				return true
			}, 1*time.Second, 50*time.Millisecond).Should(BeTrue())
		})

		objectID := func(obj client.Object) string {
			return fmt.Sprintf("%T/%s", obj, client.ObjectKeyFromObject(obj))
		}

		testDeletionUnconfirmed := func(ctx context.Context, obj client.Object) {
			Eventually(func() string {
				err := c.Delete(ctx, obj)
				return string(apierrors.ReasonForError(err))
			}, 1*time.Second, 50*time.Millisecond).Should(ContainSubstring("annotation to delete"), objectID(obj))
		}

		testDeletionConfirmed := func(ctx context.Context, obj client.Object) {
			Eventually(func() error {
				return c.Delete(ctx, obj)
			}, 1*time.Second, 50*time.Millisecond).ShouldNot(HaveOccurred(), objectID(obj))
			Eventually(func() bool {
				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
				return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
			}, 1*time.Second, 50*time.Millisecond).Should(BeTrue(), objectID(obj))
		}

		Context("custom resource definitions", func() {
			It("should admit the deletion because CRD has no protection label", func() {
				for _, obj := range crdObjects {
					// patch out default gardener.cloud/deletion-protected=true label
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetLabels(nil)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})

			It("should admit the deletion because CRD's protection label is not true", func() {
				for _, obj := range crdObjects {
					// override default gardener.cloud/deletion-protected=true label
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetLabels(deletionUnprotectedLabels)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})

			It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
				// CRDs in seed-bootstrap chart should have gardener.cloud/deletion-protected=true label by default
				for _, obj := range crdObjects {
					testDeletionUnconfirmed(ctx, obj)
				}
			})

			It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
				// CRDs in seed-bootstrap chart should have gardener.cloud/deletion-protected=true label by default
				for _, obj := range crdObjects {
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetAnnotations(deletionConfirmedAnnotations)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})
		})

		Context("extension resources", func() {
			BeforeEach(func() {
				By("creating extension test objects")
				_, err := test.EnsureTestResources(ctx, c, "testdata")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should prevent the deletion because deletion is not confirmed", func() {
				for _, obj := range extensionObjects {
					testDeletionUnconfirmed(ctx, obj)
				}
			})

			It("should admit the deletion because deletion is confirmed", func() {
				for _, obj := range extensionObjects {
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetAnnotations(deletionConfirmedAnnotations)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})
		})

		Context("other resources", func() {
			It("should not block deletion of other resources", func() {
				Expect(c.Create(ctx, podObject)).To(Succeed())
				testDeletionConfirmed(ctx, podObject)
			})
		})
	})
})
