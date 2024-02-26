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

package tokeninvalidator_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/tokeninvalidator"
)

var _ = Describe("Handler", func() {
	var (
		ctx     = context.TODO()
		log     logr.Logger
		handler *Handler

		secret *corev1.Secret
	)

	BeforeEach(func() {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{})
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		handler = &Handler{Logger: log}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{},
			Data: map[string][]byte{
				"ca.crt": []byte("key"),
				"token":  []byte("token"),
			},
		}
	})

	Describe("#Default", func() {
		It("should allow if secret data is nil", func() {
			secret.Data = nil

			Expect(handler.Default(ctx, secret)).To(Succeed())
			Expect(secret.Data["token"]).To(BeNil())
		})

		It("should invalidate the token if the secret has the consider label", func() {
			secret.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/consider": "true"}

			Expect(handler.Default(ctx, secret)).To(Succeed())
			Expect(secret.Data["token"]).To(Equal([]byte("\u0000\u0000\u0000")))
		})

		It("should delete the token key if the secret does not have the consider label and the token is invalid", func() {
			secret.Data["token"] = []byte("\u0000\u0000\u0000")

			Expect(handler.Default(ctx, secret)).To(Succeed())
			Expect(secret.Data["token"]).To(BeNil())
		})

		It("should not delete the token key if the secret does not have the consider label and the token is not invalid", func() {
			Expect(handler.Default(ctx, secret)).To(Succeed())
			Expect(secret.Data["token"]).To(Equal([]byte("token")))
		})
	})
})
