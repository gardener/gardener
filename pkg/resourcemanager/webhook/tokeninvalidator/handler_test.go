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

package tokeninvalidator_test

import (
	"context"
	"net/http"

	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/tokeninvalidator"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.TODO()
		err error

		logger logr.Logger

		decoder *admission.Decoder
		encoder runtime.Encoder
		handler admission.Handler

		request admission.Request
		secret  *corev1.Secret

		patchType = admissionv1.PatchTypeJSONPatch
	)

	BeforeEach(func() {
		logger = logzap.New(logzap.WriteTo(GinkgoWriter))

		decoder, err = admission.NewDecoder(kubernetesscheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
		encoder = &json.Serializer{}

		handler = NewHandler(logger)
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

		request = admission.Request{}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{},
			Data: map[string][]byte{
				"ca.crt": []byte("key"),
				"token":  []byte("token"),
			},
		}
	})

	Describe("#Handle", func() {
		It("should return an error because the secret cannot be decoded", func() {
			request.Object.Raw = []byte(`{]`)

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    int32(http.StatusUnprocessableEntity),
						Message: "couldn't get version/kind; json parse error: invalid character ']' looking for beginning of object key string",
					},
				},
			}))
		})

		It("should allow if secret data is nil", func() {
			secret.Data = nil

			objData, err := runtime.Encode(encoder, secret)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "data is nil",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should invalidate the token if the secret has the consider label", func() {
			secret.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/consider": "true"}

			objData, err := runtime.Encode(encoder, secret)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				Patches: []jsonpatch.JsonPatchOperation{{
					Operation: "replace",
					Path:      "/data/token",
					Value:     "AAAA",
				}},
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: &patchType,
				},
			}))
		})

		It("should delete the token key if the secret does not have the consider label and the token is invalid", func() {
			secret.Data["token"] = []byte("\u0000\u0000\u0000")

			objData, err := runtime.Encode(encoder, secret)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				Patches: []jsonpatch.JsonPatchOperation{{
					Operation: "remove",
					Path:      "/data/token",
				}},
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: &patchType,
				},
			}))
		})

		It("should not delete the token key if the secret does not have the consider label and the token is not invalid", func() {
			objData, err := runtime.Encode(encoder, secret)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				Patches: []jsonpatch.JsonPatchOperation{},
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
				},
			}))
		})
	})
})
