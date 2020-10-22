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

package webhooks_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/gardener/gardener/pkg/admissioncontroller/server/handlers/webhooks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("ValidateKubeconfigSecret", func() {
	var (
		empty = func() runtime.Object {
			return nil
		}

		configMap = func() runtime.Object {
			return &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
			}
		}

		secretTypeMeta = metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
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

	DescribeTable("Validate Kubeconfig",
		func(objFn func() runtime.Object, op admissionv1beta1.Operation, expectedAllowed bool, expectedMsg string) {
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			user := func() authenticationv1.UserInfo {
				return authenticationv1.UserInfo{
					Username: "user",
					Groups:   []string{"group"},
				}
			}
			response := httptest.NewRecorder()
			validator := NewValidateKubeconfigSecretsHandler()

			request := createHTTPRequest(objFn(), scheme, user(), op)

			validator.ServeHTTP(response, request)

			admissionReview := &admissionv1beta1.AdmissionReview{}
			Expect(decodeAdmissionResponse(response, admissionReview)).To(Succeed())
			Expect(response).Should(HaveHTTPStatus(http.StatusOK))
			Expect(admissionReview.Response).To(Not(BeNil()))
			Expect(admissionReview.Response.Allowed).To(Equal(expectedAllowed))
			if expectedMsg != "" {
				Expect(admissionReview.Response.Result).To(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Message": ContainSubstring(expectedMsg),
					})))
			}

		},
		Entry("request is empty (create)", empty, admissionv1beta1.Create, false, "missing admission request"),
		Entry("request contains configMap", configMap, admissionv1beta1.Create, false, "expect resource to be { v1 secrets}"),
		Entry("should pass because no Kubeconfig is found (create)", noKubeconfigSecret, admissionv1beta1.Create, true, emptyMessage),
		Entry("should pass because Kubeconfig is valid (create)", validKubeconfigSecret, admissionv1beta1.Create, true, emptyMessage),
		Entry("should fail because Kubeconfig is invalid (create)", invalidKubeconfigSecret, admissionv1beta1.Create, false, "exec configurations are not supported"),
		Entry("should fail because Kubeconfig has invalid content (create)", invalidKubeconfigYamlSecret, admissionv1beta1.Create, false, "cannot unmarshal string into Go value of type struct"),
		Entry("request is empty (update)", empty, admissionv1beta1.Update, false, "missing admission request"),
		Entry("should pass because no Kubeconfig is found (update)", noKubeconfigSecret, admissionv1beta1.Update, true, emptyMessage),
		Entry("should pass because Kubeconfig is valid (update)", validKubeconfigSecret, admissionv1beta1.Update, true, emptyMessage),
		Entry("should fail because Kubeconfig is invalid (update)", invalidKubeconfigSecret, admissionv1beta1.Update, false, "exec configurations are not supported"),
		Entry("should fail because Kubeconfig has invalid content (update)", invalidKubeconfigYamlSecret, admissionv1beta1.Update, false, "cannot unmarshal string into Go value of type struct"),
		Entry("should pass because operation is delete", invalidKubeconfigSecret, admissionv1beta1.Delete, true, emptyMessage),
	)
})
