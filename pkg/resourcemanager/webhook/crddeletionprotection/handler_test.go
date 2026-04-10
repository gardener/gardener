// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeletionprotection_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/crddeletionprotection"
)

var _ = Describe("handler", func() {
	Describe("#ValidateExtensionDeletion", func() {
		var (
			ctx = context.TODO()
			log logr.Logger

			request admission.Request
			decoder admission.Decoder
			handler admission.Handler
		)

		BeforeEach(func() {
			log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

			request = admission.Request{}
			request.Operation = admissionv1.Delete

			var err error
			decoder = admission.NewDecoder(kubernetes.SeedScheme)
			Expect(err).NotTo(HaveOccurred())

			handler = &Handler{Logger: log, SourceReader: fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build(), Decoder: decoder}
		})

		var (
			resources = []metav1.GroupVersionResource{
				{Group: apiextensionsv1beta1.SchemeGroupVersion.Group, Version: apiextensionsv1beta1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"},
				{Group: apiextensionsv1.SchemeGroupVersion.Group, Version: apiextensionsv1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"},
				{Group: druidcorev1alpha1.SchemeGroupVersion.Group, Version: druidcorev1alpha1.SchemeGroupVersion.Version, Resource: "etcds"},

				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupbuckets"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupentries"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "containerruntimes"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "controlplanes"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "dnsrecords"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "extensions"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "infrastructures"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "networks"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "operatingsystemconfigs"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "selfhostedshootexposures"},
				{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "workers"},
			}
			fooResource = metav1.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"}

			deletionConfirmedAnnotations = map[string]string{v1beta1constants.ConfirmationDeletion: "true"}
		)

		resourceToId := func(resource metav1.GroupVersionResource) string {
			return fmt.Sprintf("%s/%s/%s", resource.Group, resource.Version, resource.Resource)
		}

		testDeletionUnconfirmed := func(ctx context.Context, request admission.Request, resource metav1.GroupVersionResource) {
			request.Resource = resource
			expectDenied(handler.Handle(ctx, request), ContainSubstring("annotation to delete"), resourceToId(resource))
		}

		testDeletionConfirmed := func(ctx context.Context, request admission.Request, resource metav1.GroupVersionResource) {
			request.Resource = resource
			expectAllowed(handler.Handle(ctx, request), Equal(""), resourceToId(resource))
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
				expectAllowed(handler.Handle(ctx, request), ContainSubstring("not DELETE"))
				request.Operation = admissionv1.Update
				expectAllowed(handler.Handle(ctx, request), ContainSubstring("not DELETE"))
				request.Operation = admissionv1.Connect
				expectAllowed(handler.Handle(ctx, request), ContainSubstring("not DELETE"))
			})

			It("should ignore types other than CRDs + extension resources", func() {
				request.Resource = fooResource
				expectAllowed(handler.Handle(ctx, request), ContainSubstring("resource is not deletion-protected"))
			})
		})

		Context("old object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
			})

			It("should return an error because the old object cannot be decoded", func() {
				for _, resource := range resources {
					request.Name = "foo"
					request.Resource = resource
					request.OldObject = runtime.RawExtension{Raw: []byte("foo")}
					expectErrored(handler.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), ContainSubstring("invalid character"), resourceToId(resource))
				}
			})

			It("should prevent the deletion because deletion is not confirmed", func() {
				for _, resource := range resources {
					objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
					request.Name = "foo"
					request.OldObject = runtime.RawExtension{Raw: objJSON}
					testDeletionUnconfirmed(ctx, request, resource)
				}
			})

			It("should admit the deletion because deletion is confirmed", func() {
				for _, resource := range resources {
					objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, deletionConfirmedAnnotations)
					request.Name = "foo"
					request.OldObject = runtime.RawExtension{Raw: objJSON}
					testDeletionConfirmed(ctx, request, resource)
				}
			})
		})

		Context("new object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				request.Name = "foo"
			})

			It("should return an error because the new object cannot be decoded", func() {
				for _, resource := range resources {
					request.Resource = resource
					request.Object = runtime.RawExtension{Raw: []byte("foo")}

					expectErrored(handler.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), ContainSubstring("invalid character"), resourceToId(resource))
				}
			})

			It("should prevent the deletion because deletion is not confirmed", func() {
				for _, resource := range resources {
					objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
					request.Object = runtime.RawExtension{Raw: objJSON}
					testDeletionUnconfirmed(ctx, request, resource)
				}
			})

			It("should admit the deletion because deletion is confirmed", func() {
				for _, resource := range resources {
					objJSON := getObjectJSONWithLabelsAnnotations(obj, resource, nil, deletionConfirmedAnnotations)
					request.Object = runtime.RawExtension{Raw: objJSON}
					testDeletionConfirmed(ctx, request, resource)
				}
			})
		})

		Context("object must be looked up (DELETE)", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				request.Name = "foo"
				request.Namespace = "bar"
			})

			It("should return an error because the GET call failed", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(&request, obj, resource)
					request.Resource = resource
					expectErrored(handler.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), Equal("no object found in admission request"), resourceToId(resource))
				}
			})
		})

		Context("object must be looked up (DELETE COLLECTION)", func() {
			var obj *unstructured.UnstructuredList

			BeforeEach(func() {
				obj = &unstructured.UnstructuredList{}
				obj.SetKind("List")
				request.Namespace = "bar"
			})

			It("should return an error because the LIST call failed", func() {
				for _, resource := range resources {
					fakeErr := errors.New("fake")
					prepareRequestAndObjectWithResource(&request, obj, resource)
					request.Resource = resource
					obj.SetKind("List")

					c := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return fakeErr
						},
					}).Build()
					handler = &Handler{Logger: log, SourceReader: c, Decoder: decoder}

					expectErrored(handler.Handle(ctx, request), BeEquivalentTo(http.StatusInternalServerError), Equal(fakeErr.Error()), resourceToId(resource))
				}
			})

			It("should return no error because the LIST call returned 'not found'", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(&request, obj, resource)
					request.Resource = resource
					obj.SetKind("List")

					c := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return apierrors.NewNotFound(core.Resource(resource.Resource), "name")
						},
					}).Build()
					handler = &Handler{Logger: log, SourceReader: c, Decoder: decoder}

					expectAllowed(handler.Handle(ctx, request), ContainSubstring("object was not found"), resourceToId(resource))
				}
			})

			It("should prevent the deletion because deletion is not confirmed", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(&request, obj, resource)
					obj.SetKind(obj.GetKind() + "List")

					c := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, nil, nil)
							return nil
						},
					}).Build()
					handler = &Handler{Logger: log, SourceReader: c, Decoder: decoder}

					testDeletionUnconfirmed(ctx, request, resource)
				}
			})

			It("should admit the deletion because deletion is confirmed", func() {
				for _, resource := range resources {
					prepareRequestAndObjectWithResource(&request, obj, resource)
					obj.SetKind(obj.GetKind() + "List")

					c := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
							prepareObjectWithLabelsAnnotations(list, resource, nil, deletionConfirmedAnnotations)
							return nil
						},
					}).Build()
					handler = &Handler{Logger: log, SourceReader: c, Decoder: decoder}

					testDeletionConfirmed(ctx, request, resource)
				}
			})
		})
	})
})

func expectAllowed(response admission.Response, reason gomegatypes.GomegaMatcher, optionalDescription ...any) {
	Expect(response.Allowed).To(BeTrue(), optionalDescription...)
	Expect(response.Result.Message).To(reason, optionalDescription...)
}

func expectDenied(response admission.Response, reason gomegatypes.GomegaMatcher, optionalDescription ...any) {
	Expect(response.Allowed).To(BeFalse(), optionalDescription...)
	Expect(response.Result.Code).To(BeEquivalentTo(http.StatusForbidden), optionalDescription...)
	Expect(response.Result.Message).To(reason, optionalDescription...)
}

func expectErrored(response admission.Response, code, err gomegatypes.GomegaMatcher, optionalDescription ...any) {
	Expect(response.Allowed).To(BeFalse(), optionalDescription...)
	Expect(response.Result.Code).To(code, optionalDescription...)
	Expect(response.Result.Message).To(err, optionalDescription...)
}
