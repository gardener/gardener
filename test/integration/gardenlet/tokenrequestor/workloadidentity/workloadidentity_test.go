// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenrequestor_test

import (
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("WorkloadIdentity TokenRequestor tests", func() {
	var (
		resourceName string

		secret           *corev1.Secret
		workloadIdentity *securityv1alpha1.WorkloadIdentity
	)

	BeforeEach(func() {
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
			}).Should(BeNotFoundError())

			Expect(testClient.Delete(ctx, workloadIdentity)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)
			}).Should(BeNotFoundError())
		})

		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
				Annotations: map[string]string{
					"workloadidentity.security.gardener.cloud/name":      resourceName,
					"workloadidentity.security.gardener.cloud/namespace": testNamespace.Name,
				},
				Labels: map[string]string{
					"security.gardener.cloud/purpose": "workload-identity-token-requestor",
				},
			},
		}

		workloadIdentity = &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
				Labels: map[string]string{
					testID: testRunID,
				},
			},
			Spec: securityv1alpha1.WorkloadIdentitySpec{
				Audiences: []string{"foo"},
				TargetSystem: securityv1alpha1.TargetSystem{
					Type: "foocloud",
				},
			},
		}

		fakeClock.SetTime(time.Now().Round(time.Second))
	})

	It("should behave correctly when: create w/o label, update w/ label, delete w/ label", func() {
		secret.Labels = nil
		Expect(testClient.Create(ctx, secret)).To(Succeed())
		Expect(testClient.Create(ctx, workloadIdentity)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			g.Expect(secret.Data["token"]).To(BeEmpty())
		}).Should(Succeed())

		secret.Labels = map[string]string{"security.gardener.cloud/purpose": "workload-identity-token-requestor"}
		Expect(testClient.Update(ctx, secret)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			g.Expect(secret.Data["token"]).ToNot(BeEmpty())

			token, err := jwt.ParseSigned(string(secret.Data["token"]), []jose.SignatureAlgorithm{jose.ES256})
			Expect(err).ToNot(HaveOccurred())

			claims := jwt.Claims{}
			Expect(token.Claims(verificationKey, &claims)).To(Succeed())
			Expect(claims.Issuer).To(Equal("https://local.gardener.cloud"))
			Expect(claims.Audience).To(Equal(jwt.Audience{"foo"}))
			Expect(claims.Subject).To(Equal(fmt.Sprintf("gardener.cloud:workloadidentity:%s:%s:%s", testNamespace.Name, resourceName, workloadIdentity.UID)))
		}).Should(Succeed())

		Expect(testClient.Delete(ctx, secret)).To(Succeed())
	})
})
