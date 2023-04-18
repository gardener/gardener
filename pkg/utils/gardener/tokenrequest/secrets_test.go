// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package tokenrequest_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
)

var _ = Describe("Secrets", func() {
	var (
		ctx = context.TODO()

		namespace = "foo-bar"
		c         client.Client
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().Build()
	})

	Describe("#RenewAccessSecrets", func() {
		It("should remove the renew-timestamp annotation from all access secrets", func() {
			var (
				secret1 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "secret1",
						Namespace:   namespace,
						Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
					},
				}
				secret2 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "secret2",
						Namespace:   namespace,
						Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
						Labels:      map[string]string{"resources.gardener.cloud/purpose": "token-requestor"},
					},
				}
				secret3 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "secret3",
						Namespace:   namespace,
						Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
						Labels:      map[string]string{"resources.gardener.cloud/purpose": "token-requestor"},
					},
				}
			)

			Expect(c.Create(ctx, secret1)).To(Succeed())
			Expect(c.Create(ctx, secret2)).To(Succeed())
			Expect(c.Create(ctx, secret3)).To(Succeed())

			Expect(RenewAccessSecrets(ctx, c, namespace)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

			Expect(secret1.Annotations).To(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			Expect(secret2.Annotations).NotTo(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			Expect(secret3.Annotations).NotTo(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
		})
	})
})
