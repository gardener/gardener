// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedadmission_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenlogger "github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/seedadmission"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Extension CRDs", func() {
	Describe("#ValidateExtensionDeletion", func() {
		var (
			ctx     = context.Background()
			logger  = gardenlogger.NewNopLogger()
			request *admissionv1beta1.AdmissionRequest

			ctrl *gomock.Controller
			c    *mockclient.MockClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			request = &admissionv1beta1.AdmissionRequest{}
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

		It("should ignore types other than CRDs + extension resources", func() {
			request.Resource = fooResource
			Expect(ValidateExtensionDeletion(ctx, nil, logger, request)).To(Succeed())
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

					err := ValidateExtensionDeletion(ctx, nil, logger, request)
					Expect(err).To(HaveOccurred(), resourceToId(resource))
					Expect(err.Error()).To(ContainSubstring("invalid character"), resourceToId(resource))
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
						testDeletionConfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionUnprotectedLabels, nil)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, nil)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, nil, logger, request, resource)
					}
				})
			})

			Context("other resources", func() {
				It("should prevent the deletion because deletion is not confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, deletionConfirmedAnnotations)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, nil, logger, request, resource)
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

					err := ValidateExtensionDeletion(ctx, nil, logger, request)
					Expect(err).To(HaveOccurred(), resourceToId(resource))
					Expect(err.Error()).To(ContainSubstring("invalid character"), resourceToId(resource))
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
						testDeletionConfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionUnprotectedLabels, nil)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, nil)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, nil, logger, request, resource)
					}
				})
			})

			Context("other resources", func() {
				It("should prevent the deletion because deletion is not confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionUnconfirmed(ctx, nil, logger, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, deletionConfirmedAnnotations)
						request.Object = runtime.RawExtension{Raw: objJSON}
						testDeletionConfirmed(ctx, nil, logger, request, resource)
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
					prepareRequestAndObjectWithResource(request, obj, resource)
					request.Resource = resource
					request.Name = "foo"

					c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&unstructured.Unstructured{})).Return(fakeErr)

					err := ValidateExtensionDeletion(ctx, c, logger, request)
					Expect(err).To(HaveOccurred(), resourceToId(resource))
					Expect(err).To(Equal(err))
				}
			})

			It("should return no error because the GET call returned 'not found'", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(request, obj, resource)
					request.Resource = resource
					request.Name = "foo"

					c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&unstructured.Unstructured{})).Return(apierrors.NewNotFound(core.Resource(resource.Resource), "name"))

					Expect(ValidateExtensionDeletion(ctx, c, logger, request)).To(Succeed(), resourceToId(resource))
				}
			})

			Context("custom resource definitions", func() {
				BeforeEach(func() {
					request.Kind = metav1.GroupVersionKind{Kind: "CustomResourceDefinition"}
					request.Name = "foo-crd"
				})

				It("should admit the deletion because CRD has no protection label", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)

						c.EXPECT().Get(ctx, kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
							prepareObjectWithLabelsAnnotations(o, resource, nil, nil)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)

						c.EXPECT().Get(ctx, kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
							prepareObjectWithLabelsAnnotations(o, resource, deletionUnprotectedLabels, nil)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)

						c.EXPECT().Get(ctx, kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
							prepareObjectWithLabelsAnnotations(o, resource, deletionProtectedLabels, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)

						c.EXPECT().Get(ctx, kutil.Key(request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
							prepareObjectWithLabelsAnnotations(o, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
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
						prepareRequestAndObjectWithResource(request, obj, resource)

						c.EXPECT().Get(ctx, kutil.Key(request.Namespace, request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
							prepareObjectWithLabelsAnnotations(o, resource, nil, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						prepareRequestAndObjectWithResource(request, obj, resource)

						c.EXPECT().Get(ctx, kutil.Key(request.Namespace, request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
							prepareObjectWithLabelsAnnotations(o, resource, nil, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
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
					prepareRequestAndObjectWithResource(request, obj, resource)
					request.Resource = resource
					obj.SetKind("List")

					c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).Return(fakeErr)

					err := ValidateExtensionDeletion(ctx, c, logger, request)
					Expect(err).To(HaveOccurred(), resourceToId(resource))
					Expect(err).To(Equal(err))
				}
			})

			It("should return no error because the LIST call returned 'not found'", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(request, obj, resource)
					request.Resource = resource
					obj.SetKind("List")

					c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).Return(apierrors.NewNotFound(core.Resource(resource.Resource), "name"))

					Expect(ValidateExtensionDeletion(ctx, c, logger, request)).To(Succeed(), resourceToId(resource))
				}
			})

			Context("custom resource definitions", func() {
				BeforeEach(func() {
					request.Kind = metav1.GroupVersionKind{Kind: "CustomResourceDefinition"}
				})

				It("should admit the deletion because CRD has no protection label", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, o runtime.Object, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(o, resource, nil, nil)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is not true", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, o runtime.Object, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(o, resource, deletionUnprotectedLabels, nil)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, o runtime.Object, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(o, resource, deletionProtectedLabels, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
					for _, resource := range crdResources {
						prepareRequestAndObjectWithResource(request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, o runtime.Object, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(o, resource, deletionProtectedLabels, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
					}
				})
			})

			Context("other resources", func() {
				BeforeEach(func() {
					request.Namespace = "bar"
				})

				It("should prevent the deletion because deletion is not confirmed", func() {
					for _, resource := range otherResources {
						prepareRequestAndObjectWithResource(request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, o runtime.Object, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(o, resource, nil, nil)
							return nil
						})

						testDeletionUnconfirmed(ctx, c, logger, request, resource)
					}
				})

				It("should admit the deletion because deletion is confirmed", func() {
					for _, resource := range otherResources {
						prepareRequestAndObjectWithResource(request, obj, resource)
						obj.SetKind(obj.GetKind() + "List")

						c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, o runtime.Object, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(o, resource, nil, deletionConfirmedAnnotations)
							return nil
						})

						testDeletionConfirmed(ctx, c, logger, request, resource)
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

func prepareRequestAndObjectWithResource(request *admissionv1beta1.AdmissionRequest, obj unstructuredInterface, resource metav1.GroupVersionResource) {
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

func testDeletionUnconfirmed(
	ctx context.Context,
	c *mockclient.MockClient,
	logger *logrus.Logger,
	request *admissionv1beta1.AdmissionRequest,
	resource metav1.GroupVersionResource,
) {
	request.Resource = resource
	err := ValidateExtensionDeletion(ctx, c, logger, request)
	Expect(err).To(HaveOccurred(), resourceToId(resource))
	Expect(err.Error()).To(ContainSubstring("annotation to delete"), resourceToId(resource))
}

func testDeletionConfirmed(
	ctx context.Context,
	c *mockclient.MockClient,
	logger *logrus.Logger,
	request *admissionv1beta1.AdmissionRequest,
	resource metav1.GroupVersionResource,
) {
	request.Resource = resource
	Expect(ValidateExtensionDeletion(ctx, c, logger, request)).To(Succeed(), resourceToId(resource))
}
