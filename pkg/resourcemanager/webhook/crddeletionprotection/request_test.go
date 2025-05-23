// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeletionprotection_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/crddeletionprotection"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("admission", func() {
	var (
		ctx     = context.Background()
		request admission.Request
		decoder admission.Decoder

		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		request = admission.Request{}

		var err error
		decoder = admission.NewDecoder(kubernetes.SeedScheme)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#getRequestObject", func() {
		resource := metav1.GroupVersionResource{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: "pods"}

		Context("when old object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				request.Name = resource.Resource

				obj = &unstructured.Unstructured{}
				obj.SetAPIVersion(fmt.Sprintf("%s/%s", resource.Group, resource.Version))
				obj.SetKind(resource.Resource)
			})

			It("should return an error because the old object cannot be decoded", func() {
				request.OldObject = runtime.RawExtension{Raw: []byte("foo")}

				_, err := ExtractRequestObject(ctx, c, decoder, request, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid character"))
			})

			It("should return the old object", func() {
				objJSON, err := json.Marshal(obj)
				Expect(err).ToNot(HaveOccurred())

				request.OldObject = runtime.RawExtension{Raw: objJSON}

				result, err := ExtractRequestObject(ctx, c, decoder, request, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal(resource.Resource))
			})
		})

		Context("when new object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				request.Name = resource.Resource

				obj = &unstructured.Unstructured{}
				obj.SetAPIVersion(fmt.Sprintf("%s/%s", resource.Group, resource.Version))
				obj.SetKind(resource.Resource)
			})

			It("should return an error because the new object cannot be decoded", func() {
				request.Object = runtime.RawExtension{Raw: []byte("foo")}

				_, err := ExtractRequestObject(ctx, c, decoder, request, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid character"))
			})

			It("should return the new object", func() {
				objJSON, err := json.Marshal(obj)
				Expect(err).ToNot(HaveOccurred())

				request.Object = runtime.RawExtension{Raw: objJSON}

				result, err := ExtractRequestObject(ctx, c, decoder, request, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal(resource.Resource))
			})
		})

		Context("when object is not send by API server", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				request.Resource = resource
				request.Name = "machine-controller-manager"
				request.Namespace = "shoot--dev--test"
				request.Kind.Group = resource.Group
				request.Kind.Version = resource.Version
				obj.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
				obj.SetKind(request.Kind.Kind)
			})

			It("should return an error because the GET call failed", func() {
				_, err := ExtractRequestObject(ctx, c, decoder, request, nil)
				Expect(err).Should(MatchError("no object found in admission request"))
			})
		})

		Context("when object list must be looked up", func() {
			var (
				obj     *unstructured.UnstructuredList
				objJSON = []byte("{}")
			)

			BeforeEach(func() {
				obj = &unstructured.UnstructuredList{}
				request.Resource = resource
				request.Namespace = "shoot--dev--test"
				request.Kind.Group = resource.Group
				request.Kind.Version = resource.Version
				// Old object is set when deletion happens https://github.com/kubernetes/kubernetes/pull/76346.
				request.OldObject = runtime.RawExtension{Raw: objJSON}
				obj.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
				obj.SetKind(request.Kind.Kind + "List")
			})

			It("should return an error because the LIST call failed", func() {
				fakeErr := errors.New("fake")

				listOp := client.InNamespace(request.Namespace)

				c.EXPECT().List(ctx, obj, listOp).Return(fakeErr)

				_, err := ExtractRequestObject(ctx, c, decoder, request, listOp)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(err))
			})

			It("should return the looked up resource", func() {
				listOp := client.InNamespace(request.Namespace)

				c.EXPECT().List(ctx, obj, listOp).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					ob, ok := list.(*unstructured.UnstructuredList)
					if !ok {
						return fmt.Errorf("Error casting %v to UnstructuredList object", list)
					}
					ob.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
					ob.SetKind(request.Kind.Kind + "List")
					return nil
				})

				result, err := ExtractRequestObject(ctx, c, decoder, request, listOp)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal("List"))
			})
		})
	})
})
