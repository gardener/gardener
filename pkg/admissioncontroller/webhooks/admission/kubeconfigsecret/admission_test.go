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

package kubeconfigsecret_test

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/kubeconfigsecret"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("handler", func() {
	var (
		ctx    = context.TODO()
		logger logr.Logger

		request admission.Request
		decoder *admission.Decoder
		handler admission.Handler

		testEncoder runtime.Encoder

		statusCodeAllowed       int32 = http.StatusOK
		statusCodeInvalid       int32 = http.StatusUnprocessableEntity
		statusCodeInternalError int32 = http.StatusInternalServerError

		secretTypeMeta = metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}

		configMap = func() runtime.Object {
			return &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
			}
		}

		noKubeconfigSecret = func() runtime.Object {
			return &corev1.Secret{
				TypeMeta: secretTypeMeta,
				Data: map[string][]byte{
					"foo": {},
				},
			}
		}

		validKubeconfig = `
---
apiVersion: v1
kind: Config
current-context: local-garden
clusters:
- name: local-garden
  cluster:
    certificate-authority-data: Z2FyZGVuZXIK
    server: https://localhost:2443
contexts:
- name: local-garden
  context:
    cluster: local-garden
    user: local-garden
users:
- name: local-garden
  user:
    client-certificate-data: Z2FyZGVuZXIK
    client-key-data: Z2FyZGVuZXIK
`

		validKubeconfigSecret = func() runtime.Object {
			return &corev1.Secret{
				TypeMeta: secretTypeMeta,
				Data: map[string][]byte{
					"kubeconfig": []byte(validKubeconfig),
				},
			}
		}

		malformedKubeconfigSecret = func() runtime.Object {
			return &corev1.Secret{
				TypeMeta: secretTypeMeta,
				Data: map[string][]byte{
					"kubeconfig": []byte(`foobar`),
				},
			}
		}

		invalidKubeconfig = `
---
apiVersion: v1
kind: Config
current-context: local-garden
clusters:
- name: local-garden
  cluster:
    certificate-authority-data: Z2FyZGVuZXIK
    server: https://localhost:2443
contexts:
- name: local-garden
  context:
    cluster: local-garden
    user: local-garden
users:
- name: local-garden
  user:
    exec:
      command: /bin/sh
`

		invalidKubeconfigSecret = func() runtime.Object {
			return &corev1.Secret{
				TypeMeta: secretTypeMeta,
				Data: map[string][]byte{
					"kubeconfig": []byte(invalidKubeconfig),
				},
			}
		}

		invalidKubeconfigYamlSecret = func() runtime.Object {
			return &corev1.Secret{
				TypeMeta: secretTypeMeta,
				Data: map[string][]byte{
					"kubeconfig": []byte("foo"),
				},
			}
		}
	)

	BeforeEach(func() {
		logger = logzap.New(logzap.WriteTo(GinkgoWriter))

		var err error
		decoder, err = admission.NewDecoder(kubernetes.GardenScheme)
		Expect(err).NotTo(HaveOccurred())

		handler = New(logger)
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

		testEncoder = &json.Serializer{}
		request = admission.Request{}
		request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
	})

	test := func(objFn func() runtime.Object, op admissionv1.Operation, expectedAllowed bool, expectedStatusCode int32, expectedMsg string) {
		request.Operation = op

		if obj := objFn(); obj != nil {
			objData, err := runtime.Encode(testEncoder, objFn())
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData
		}

		response := handler.Handle(ctx, request)
		Expect(response).To(Not(BeNil()))
		Expect(response.Allowed).To(Equal(expectedAllowed))
		Expect(response.Result.Code).To(Equal(expectedStatusCode))
		if expectedMsg != "" {
			Expect(response.Result.Message).To(ContainSubstring(expectedMsg))
		}
	}

	Context("ignored requests", func() {
		It("should ignore other operations than CREATE and UPDATE", func() {
			test(validKubeconfigSecret, admissionv1.Delete, true, statusCodeAllowed, "neither CREATE nor UPDATE")
			test(validKubeconfigSecret, admissionv1.Connect, true, statusCodeAllowed, "neither CREATE nor UPDATE")
		})

		It("should ignore other resources than Pods", func() {
			request.Kind = metav1.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"}
			test(validKubeconfigSecret, admissionv1.Create, true, statusCodeAllowed, "not corev1.Secret")
		})

		It("should ignore subresources", func() {
			request.SubResource = "logs"
			test(validKubeconfigSecret, admissionv1.Create, true, statusCodeAllowed, "subresource")
		})
	})

	It("should pass because no Kubeconfig is found (create)", func() {
		test(noKubeconfigSecret, admissionv1.Create, true, statusCodeAllowed, "")
	})

	It("should pass because Kubeconfig is valid (create)", func() {
		test(validKubeconfigSecret, admissionv1.Create, true, statusCodeAllowed, "")
	})

	It("should fail because secret cannot be decoded (create)", func() {
		test(configMap, admissionv1.Create, false, statusCodeInternalError, "unable to decode")
	})

	It("should fail because Kubeconfig is malformed (create)", func() {
		test(malformedKubeconfigSecret, admissionv1.Create, false, statusCodeInvalid, "json parse error")
	})

	It("should fail because Kubeconfig is invalid (create)", func() {
		test(invalidKubeconfigSecret, admissionv1.Create, false, statusCodeInvalid, "exec configurations are not supported")
	})

	It("should fail because Kubeconfig has invalid content (create)", func() {
		test(invalidKubeconfigYamlSecret, admissionv1.Create, false, statusCodeInvalid, "cannot unmarshal string into Go value of type struct")
	})

	It("should pass because no Kubeconfig is found (update)", func() {
		test(noKubeconfigSecret, admissionv1.Update, true, statusCodeAllowed, "")
	})

	It("should pass because Kubeconfig is valid (update)", func() {
		test(validKubeconfigSecret, admissionv1.Update, true, statusCodeAllowed, "")
	})

	It("should fail because secret cannot be decoded (update)", func() {
		test(configMap, admissionv1.Update, false, statusCodeInternalError, "unable to decode")
	})

	It("should fail because Kubeconfig is malformed (update)", func() {
		test(malformedKubeconfigSecret, admissionv1.Update, false, statusCodeInvalid, "json parse error")
	})

	It("should fail because Kubeconfig is invalid (update)", func() {
		test(invalidKubeconfigSecret, admissionv1.Update, false, statusCodeInvalid, "exec configurations are not supported")
	})

	It("should fail because Kubeconfig has invalid content (update)", func() {
		test(invalidKubeconfigYamlSecret, admissionv1.Update, false, statusCodeInvalid, "cannot unmarshal string into Go value of type struct")
	})

	It("should pass because operation is delete", func() {
		test(invalidKubeconfigSecret, admissionv1.Delete, true, statusCodeAllowed, "")
	})
})
