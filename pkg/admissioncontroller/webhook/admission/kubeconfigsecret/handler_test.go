// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/kubeconfigsecret"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("handler", func() {
	var (
		ctx     = context.TODO()
		log     logr.Logger
		handler *Handler

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
		ctx = admission.NewContextWithRequest(ctx, admission.Request{})
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		handler = &Handler{Logger: log}
	})

	test := func(objFn func() runtime.Object, matcher gomegatypes.GomegaMatcher) {
		Expect(handler.ValidateCreate(ctx, objFn())).To(matcher)
		Expect(handler.ValidateUpdate(ctx, nil, objFn())).To(matcher)
	}

	It("should pass because no Kubeconfig is found (create)", func() {
		test(noKubeconfigSecret, Succeed())
	})

	It("should pass because Kubeconfig is valid (create)", func() {
		test(validKubeconfigSecret, Succeed())
	})

	It("should fail because Kubeconfig is malformed (create)", func() {
		test(malformedKubeconfigSecret, MatchError(ContainSubstring("json parse error")))
	})

	It("should fail because Kubeconfig is invalid (create)", func() {
		test(invalidKubeconfigSecret, MatchError(ContainSubstring("exec configurations are not supported")))
	})

	It("should fail because Kubeconfig has invalid content (create)", func() {
		test(invalidKubeconfigYamlSecret, MatchError(ContainSubstring("cannot unmarshal string into Go value of type struct")))
	})

	It("should pass because operation is delete", func() {
		Expect(handler.ValidateDelete(ctx, nil)).To(Succeed())
	})
})
