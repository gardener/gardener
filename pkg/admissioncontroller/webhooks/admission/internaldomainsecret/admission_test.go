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

package internaldomainsecret_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/internaldomainsecret"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("handler", func() {
	var (
		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake err")
		logger  logr.Logger

		request admission.Request
		handler admission.Handler

		ctrl       *gomock.Controller
		mockReader *mockclient.MockReader

		statusCodeAllowed       int32 = http.StatusOK
		statusCodeBadRequest    int32 = http.StatusBadRequest
		statusCodeForbidden     int32 = http.StatusForbidden
		statusCodeInternalError int32 = http.StatusInternalServerError

		namespaceName     = "foo"
		shootMetadataList *metav1.PartialObjectMetadataList
	)

	BeforeEach(func() {
		logger = logzap.New(logzap.WriteTo(GinkgoWriter))

		ctrl = gomock.NewController(GinkgoT())
		mockReader = mockclient.NewMockReader(ctrl)

		shootMetadataList = &metav1.PartialObjectMetadataList{}
		shootMetadataList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))

		decoder, err := admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		handler = New(logger)
		Expect(inject.APIReaderInto(mockReader, handler)).To(BeTrue())
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

		request = admission.Request{}
		request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	test := func(op admissionv1.Operation, expectedAllowed bool, expectedStatusCode int32, expectedMsg string) {
		request.Operation = op
		request.Name = namespaceName

		response := handler.Handle(ctx, request)
		Expect(response).To(Not(BeNil()))
		Expect(response.Allowed).To(Equal(expectedAllowed))
		Expect(response.Result.Code).To(Equal(expectedStatusCode))
		if expectedMsg != "" {
			Expect(response.Result.Message).To(ContainSubstring(expectedMsg))
		}
	}

	Context("ignored requests", func() {
		It("should ignore other operations than CREATE, UPDATE, DELETE", func() {
			test(admissionv1.Connect, true, statusCodeAllowed, "unknown operation")
		})

		It("should ignore other resources than Secrets", func() {
			request.Kind = metav1.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"}
			test(admissionv1.Delete, true, statusCodeAllowed, "not corev1.Secret")
		})

		It("should ignore subresources", func() {
			request.SubResource = "finalize"
			test(admissionv1.Delete, true, statusCodeAllowed, "subresource")
		})
	})

	Context("create", func() {
		It("should fail because the check for other internal domain secrets failed", func() {
			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain}, client.Limit(1)).Return(fakeErr)

			test(admissionv1.Create, false, statusCodeInternalError, fakeErr.Error())
		})

		It("should fail because another internal domain secret exists", func() {
			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain}, client.Limit(1)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
				return nil
			})

			test(admissionv1.Create, false, statusCodeForbidden, "")
		})

		It("should fail because the object cannot be decoded", func() {
			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain}, client.Limit(1))

			test(admissionv1.Create, false, statusCodeInternalError, "")
		})

		It("should fail because the secret misses domain info", func() {
			request.Object.Raw = encode(&corev1.Secret{})

			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain}, client.Limit(1))

			test(admissionv1.Create, false, statusCodeBadRequest, "")
		})

		It("should pass because no other internal domain secret exists", func() {
			request.Object.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "foo",
						"dns.gardener.cloud/domain":   "bar",
					},
				},
			})

			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain}, client.Limit(1))

			test(admissionv1.Create, true, statusCodeAllowed, "no internal domain secrets exist")
		})
	})

	Context("update", func() {
		It("should fail because the object cannot be decoded", func() {
			test(admissionv1.Update, false, statusCodeInternalError, "")
		})

		It("should fail because the old object cannot be decoded", func() {
			request.Object.Raw = encode(&corev1.Secret{})

			test(admissionv1.Update, false, statusCodeInternalError, "")
		})

		It("should fail because the secret misses domain info", func() {
			request.Object.Raw = encode(&corev1.Secret{})
			request.OldObject.Raw = encode(&corev1.Secret{})

			test(admissionv1.Update, false, statusCodeBadRequest, "")
		})

		It("should fail because the old secret misses domain info", func() {
			request.Object.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "foo",
						"dns.gardener.cloud/domain":   "bar",
					},
				},
			})
			request.OldObject.Raw = encode(&corev1.Secret{})

			test(admissionv1.Update, false, statusCodeBadRequest, "")
		})

		It("should forbid because the domain is changed but shoot listing failed", func() {
			request.Object.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "foo",
						"dns.gardener.cloud/domain":   "bar",
					},
				},
			})
			request.OldObject.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "bar",
						"dns.gardener.cloud/domain":   "foo",
					},
				},
			})

			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).Return(fakeErr)

			test(admissionv1.Update, false, statusCodeInternalError, fakeErr.Error())
		})

		It("should forbid because the domain is changed but shoots exist", func() {
			request.Object.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "foo",
						"dns.gardener.cloud/domain":   "bar",
					},
				},
			})
			request.OldObject.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "bar",
						"dns.gardener.cloud/domain":   "foo",
					},
				},
			})

			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
				return nil
			})

			test(admissionv1.Update, false, statusCodeForbidden, "")
		})

		It("should allow because the domain is changed but no shoots exist", func() {
			request.Object.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "foo",
						"dns.gardener.cloud/domain":   "bar",
					},
				},
			})
			request.OldObject.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "bar",
						"dns.gardener.cloud/domain":   "foo",
					},
				},
			})

			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1))

			test(admissionv1.Update, true, statusCodeAllowed, "domain didn't change or no shoot exists")
		})

		It("should allow because the domain is not changed", func() {
			request.Object.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "foo",
						"dns.gardener.cloud/domain":   "bar",
					},
				},
			})
			request.OldObject.Raw = encode(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "baz",
						"dns.gardener.cloud/domain":   "bar",
					},
				},
			})

			test(admissionv1.Update, true, statusCodeAllowed, "domain didn't change or no shoot exists")
		})
	})

	Context("delete", func() {
		It("should fail because the shoot listing fails", func() {
			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).Return(fakeErr)

			test(admissionv1.Delete, false, statusCodeInternalError, fakeErr.Error())
		})

		It("should fail because at least one shoot exists", func() {
			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
				return nil
			})

			test(admissionv1.Delete, false, statusCodeForbidden, "")
		})

		It("should pass because no shoots exist", func() {
			mockReader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1))

			test(admissionv1.Delete, true, statusCodeAllowed, "no shoot exists")
		})
	})
})

func encode(obj runtime.Object) []byte {
	raw, _ := json.Marshal(obj)
	return raw
}
