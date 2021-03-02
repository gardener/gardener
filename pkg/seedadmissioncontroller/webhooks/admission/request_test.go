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

package admission_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("admission", func() {
	var (
		ctx     = context.Background()
		request admission.Request
		decoder *admission.Decoder

		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		request = admission.Request{}

		var err error
		decoder, err = admission.NewDecoder(kubernetes.SeedScheme)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#getRequestObject", func() {
		resource := metav1.GroupVersionResource{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: "pods"}

		Context("Old object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				obj.SetAPIVersion(fmt.Sprintf("%s/%s", resource.Group, resource.Version))
				obj.SetKind(resource.Resource)
			})

			It("should return an error because the old object cannot be decoded", func() {
				request.OldObject = runtime.RawExtension{Raw: []byte("foo")}

				_, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid character"))
			})

			It("should return the old object", func() {
				objJSON, err := json.Marshal(obj)
				Expect(err).ToNot(HaveOccurred())

				request.OldObject = runtime.RawExtension{Raw: objJSON}

				result, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal(resource.Resource))
			})
		})

		Context("New object is set", func() {
			var obj *unstructured.Unstructured

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				obj.SetAPIVersion(fmt.Sprintf("%s/%s", resource.Group, resource.Version))
				obj.SetKind(resource.Resource)
			})

			It("should return an error because the new object cannot be decoded", func() {
				request.Object = runtime.RawExtension{Raw: []byte("foo")}

				_, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid character"))
			})

			It("should return the new object", func() {
				objJSON, err := json.Marshal(obj)
				Expect(err).ToNot(HaveOccurred())

				request.Object = runtime.RawExtension{Raw: objJSON}

				result, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal(resource.Resource))
			})
		})

		Context("object must be looked up", func() {
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
				fakeErr := errors.New("fake")

				c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&unstructured.Unstructured{})).Return(fakeErr)

				_, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(err))
			})

			It("Shoul return the looked up resource", func() {
				c.EXPECT().Get(ctx, kutil.Key(request.Namespace, request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
					ob, ok := obj.(*unstructured.Unstructured)
					if !ok {
						return fmt.Errorf("Error casting %v to Unstructured object", obj)
					}
					ob.SetAPIVersion(fmt.Sprintf("%s/%s", resource.Group, resource.Version))
					ob.SetKind(resource.Resource)
					return nil
				})

				result, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal(resource.Resource))
			})
		})

		Context("object list must be looked up", func() {
			var obj *unstructured.UnstructuredList

			BeforeEach(func() {
				obj = &unstructured.UnstructuredList{}
				request.Resource = resource
				request.Namespace = "shoot--dev--test"
				request.Kind.Group = resource.Group
				request.Kind.Version = resource.Version
				obj.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
				obj.SetKind(request.Kind.Kind + "List")
			})

			It("should return an error because the GET call failed", func() {
				fakeErr := errors.New("fake")

				c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).Return(fakeErr)

				_, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(err))
			})

			It("should return the looked up resource", func() {
				c.EXPECT().List(ctx, obj, client.InNamespace(request.Namespace)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					ob, ok := list.(*unstructured.UnstructuredList)
					if !ok {
						return fmt.Errorf("Error casting %v to UnstructuredList object", list)
					}
					ob.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
					ob.SetKind(request.Kind.Kind + "List")
					return nil
				})

				result, err := ExtractRequestObject(ctx, c, decoder, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal("List"))
			})
		})
	})
})
