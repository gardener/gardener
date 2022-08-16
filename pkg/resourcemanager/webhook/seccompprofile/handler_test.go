// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seccompprofile_test

import (
	"context"
	"net/http"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/seccompprofile"
	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		ctx     = context.TODO()
		encoder runtime.Encoder

		log     logr.Logger
		request admission.Request
		pod     *corev1.Pod

		handler seccompprofile.Handler
	)

	BeforeEach(func() {
		decoder, err := admission.NewDecoder(kubernetesscheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
		encoder = &json.Serializer{}

		log = logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		request = admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Kind:      metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"},
				Operation: admissionv1.Create,
			},
		}
		pod = &corev1.Pod{
			Spec: corev1.PodSpec{},
		}

		handler = seccompprofile.NewHandler(log)
		Expect(admission.InjectDecoderInto(decoder, &handler)).To(BeTrue())
	})

	Describe("#Handle", func() {
		It("should allow", func() {
			request.Operation = admissionv1.Update

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "only 'create' operation is handled",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should allow when resource is not corev1.Pod", func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Kind: "ConfigMap", Version: "v1"}

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "resource is not corev1.Pod",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should allow when subresource is specified", func() {
			request.SubResource = "status"

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "subresources on pods are not supported",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should return an error because the pod cannot be decoded", func() {
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

		It("should not patch the seccomp profile name when the pod already specifies seccomp profile", func() {
			pod.Spec.SecurityContext = &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeUnconfined,
				},
			}
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "seccomp profile is explicitly specified",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should default the seccomp profile type when it is not explicitly specified", func() {
			pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)
			patchType := admissionv1.PatchTypeJSONPatch

			Expect(response.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
				Allowed:   true,
				PatchType: &patchType,
			}))

			Expect(response.Patches).To(ConsistOf(
				jsonpatch.JsonPatchOperation{
					Operation: "add",
					Path:      "/spec/securityContext/seccompProfile",
					Value: map[string]interface{}{
						"type": "RuntimeDefault",
					},
				},
			))
		})
	})
})
