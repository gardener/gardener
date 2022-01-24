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

package projectedtokenmount_test

import (
	"context"
	"net/http"

	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/projectedtokenmount"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.TODO()
		err error

		decoder    *admission.Decoder
		encoder    runtime.Encoder
		fakeClient client.Client
		handler    admission.Handler

		request        admission.Request
		pod            *corev1.Pod
		serviceAccount *corev1.ServiceAccount

		namespace          = "some-namespace"
		serviceAccountName = "some-service-account"
		expirationSeconds  int64

		patchType = admissionv1.PatchTypeJSONPatch
	)

	BeforeEach(func() {
		decoder, err = admission.NewDecoder(kubernetesscheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
		encoder = &json.Serializer{}

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

		handler = NewHandler(fakeClient, expirationSeconds)
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

		request = admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Namespace: namespace,
			},
		}
		pod = &corev1.Pod{
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				Containers:         []corev1.Container{{}, {}},
			},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace,
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		expirationSeconds = 1337
	})

	Describe("#Handle", func() {
		It("should allow because operation is not 'create'", func() {
			request.Operation = admissionv1.Update

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "only 'create' operation is handled",
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

		DescribeTable("should not mutate because preconditions are not met",
			func(mutatePod func(), expectedReason string) {
				mutatePod()

				objData, err := runtime.Encode(encoder, pod)
				Expect(err).NotTo(HaveOccurred())
				request.Object.Raw = objData

				Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Reason: metav1.StatusReason(expectedReason),
							Code:   http.StatusOK,
						},
					},
				}))
			},

			Entry("service account name is empty",
				func() {
					pod.Spec.ServiceAccountName = ""
				},
				"service account not specified or defaulted",
			),

			Entry("service account name is default",
				func() {
					pod.Spec.ServiceAccountName = "default"
				},
				"service account not specified or defaulted",
			),
		)

		It("should return an error because the service account cannot be read", func() {
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    int32(http.StatusInternalServerError),
						Message: "serviceaccounts \"" + serviceAccountName + "\" not found",
					},
				},
			}))
		})

		DescribeTable("should not mutate because service account's preconditions are not met",
			func(mutate func()) {
				mutate()

				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

				objData, err := runtime.Encode(encoder, pod)
				Expect(err).NotTo(HaveOccurred())
				request.Object.Raw = objData

				Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Reason: "auto-mounting service account token is not disabled on ServiceAccount level",
							Code:   http.StatusOK,
						},
					},
				}))
			},

			Entry("ServiceAccount's automountServiceAccountToken=nil", func() {
				serviceAccount.AutomountServiceAccountToken = nil
			}),
			Entry("ServiceAccount's automountServiceAccountToken=true", func() {
				serviceAccount.AutomountServiceAccountToken = pointer.Bool(true)
			}),
		)

		It("should not mutate because pod explicitly disables the service account mount", func() {
			pod.Spec.AutomountServiceAccountToken = pointer.Bool(false)

			Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "Pod explicitly disables a service account token mount",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should not mutate because pod already has a projected token volume", func() {
			Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

			pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{Name: "kube-api-access-2138h"})

			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "pod already has service account volume mount",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		Context("should mutate", func() {
			AfterEach(func() {
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

				objData, err := runtime.Encode(encoder, pod)
				Expect(err).NotTo(HaveOccurred())
				request.Object.Raw = objData

				response := handler.Handle(ctx, request)

				Expect(response.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: &patchType,
				}))
				Expect(response.Patches).To(ConsistOf(
					jsonpatch.JsonPatchOperation{
						Operation: "add",
						Path:      "/spec/securityContext",
						Value: map[string]interface{}{
							"fsGroup": float64(65534),
						},
					},
					jsonpatch.JsonPatchOperation{
						Operation: "add",
						Path:      "/spec/volumes",
						Value: []interface{}{
							map[string]interface{}{
								"name": "kube-api-access-gardener",
								"projected": map[string]interface{}{
									"defaultMode": float64(420),
									"sources": []interface{}{
										map[string]interface{}{
											"serviceAccountToken": map[string]interface{}{
												"expirationSeconds": float64(expirationSeconds),
												"path":              "token",
											},
										},
										map[string]interface{}{
											"configMap": map[string]interface{}{
												"name": "kube-root-ca.crt",
												"items": []interface{}{
													map[string]interface{}{
														"key":  "ca.crt",
														"path": "ca.crt",
													},
												},
											},
										},
										map[string]interface{}{
											"downwardAPI": map[string]interface{}{
												"items": []interface{}{
													map[string]interface{}{
														"path": "namespace",
														"fieldRef": map[string]interface{}{
															"apiVersion": "v1",
															"fieldPath":  "metadata.namespace",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					jsonpatch.JsonPatchOperation{
						Operation: "add",
						Path:      "/spec/containers/0/volumeMounts",
						Value: []interface{}{
							map[string]interface{}{
								"name":      "kube-api-access-gardener",
								"readOnly":  true,
								"mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
							},
						},
					},
					jsonpatch.JsonPatchOperation{
						Operation: "add",
						Path:      "/spec/containers/1/volumeMounts",
						Value: []interface{}{
							map[string]interface{}{
								"name":      "kube-api-access-gardener",
								"readOnly":  true,
								"mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
							},
						},
					},
				))
			})

			It("normal case", func() {})

			It("with overridden expiration seconds", func() {
				expirationSeconds = 8998
				pod.Annotations = map[string]string{"projected-token-mount.resources.gardener.cloud/expiration-seconds": "8998"}
			})
		})
	})
})
