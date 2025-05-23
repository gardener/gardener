// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Service accounts", func() {
	var (
		ctx    = context.TODO()
		logger logr.Logger

		kubeAPIServerNamespace = "shoot--foo--bar"

		runtimeClient      client.Client
		targetClient       client.Client
		fakeSecretsManager secretsmanager.Interface
	)

	BeforeEach(func() {
		logger = logr.Discard()

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		targetClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeSecretsManager = fakesecretsmanager.New(runtimeClient, kubeAPIServerNamespace)
	})

	Context("service account signing key secret rotation", func() {
		var (
			namespace1, namespace2 *corev1.Namespace
			sa1, sa2, sa3          *corev1.ServiceAccount
			suffix                 = "-4c6b7a"
		)

		BeforeEach(func() {
			namespace1 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}
			namespace2 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}}

			Expect(targetClient.Create(ctx, namespace1)).To(Succeed())
			Expect(targetClient.Create(ctx, namespace2)).To(Succeed())

			sa1 = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: namespace1.Name},
				Secrets:    []corev1.ObjectReference{{Name: "sa1secret1"}},
			}
			sa2 = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "sa2", Namespace: namespace2.Name},
				Secrets:    []corev1.ObjectReference{{Name: "sa2-token" + suffix}, {Name: "sa2secret1"}},
			}
			sa3 = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "sa3", Namespace: namespace2.Name, Labels: map[string]string{"credentials.gardener.cloud/key-name": "service-account-key-current"}},
				Secrets:    []corev1.ObjectReference{{Name: "sa3secret1"}},
			}

			Expect(targetClient.Create(ctx, sa1)).To(Succeed())
			Expect(targetClient.Create(ctx, sa2)).To(Succeed())
			Expect(targetClient.Create(ctx, sa3)).To(Succeed())
		})

		Describe("#CreateNewServiceAccountSecrets", func() {
			It("should create new service account secrets and make them the first in the list", func() {
				Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "service-account-key-current", Namespace: kubeAPIServerNamespace}})).To(Succeed())

				Expect(CreateNewServiceAccountSecrets(ctx, logger, targetClient, fakeSecretsManager)).To(Succeed())

				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(sa1), sa1)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(sa2), sa2)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(sa3), sa3)).To(Succeed())

				Expect(sa1.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "service-account-key-current"))
				Expect(sa2.Labels).NotTo(HaveKeyWithValue("credentials.gardener.cloud/key-name", "service-account-key-current"))
				Expect(sa3.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "service-account-key-current"))
				Expect(sa1.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa1-token" + suffix}, corev1.ObjectReference{Name: "sa1secret1"}))
				Expect(sa2.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa2-token" + suffix}, corev1.ObjectReference{Name: "sa2secret1"}))
				Expect(sa3.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa3secret1"}))

				sa1Secret := &corev1.Secret{}
				Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: sa1.Namespace, Name: "sa1-token" + suffix}, sa1Secret)).To(Succeed())
				verifyCreatedSATokenSecret(sa1Secret, sa1.Name)
			})
		})

		Describe("#DeleteOldServiceAccountSecrets", func() {
			It("should delete old service account secrets", func() {
				now := time.Now()

				By("Create old ServiceAccount secrets")
				Expect(targetClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              "sa1secret1",
					Namespace:         sa1.Namespace,
					CreationTimestamp: metav1.Time{Time: now},
				}})).To(Succeed())
				Expect(targetClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              "sa2secret1",
					Namespace:         sa2.Namespace,
					CreationTimestamp: metav1.Time{Time: now},
				}})).To(Succeed())
				Expect(targetClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              "sa3secret1",
					Namespace:         sa3.Namespace,
					CreationTimestamp: metav1.Time{Time: now},
				}})).To(Succeed())

				By("Set time of last credentials rotation")
				lastInitiationFinishedTime := now.Add(time.Minute)

				By("Create new ServiceAccount secret")
				Expect(targetClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              sa2.Secrets[0].Name,
					Namespace:         sa2.Namespace,
					CreationTimestamp: metav1.Time{Time: lastInitiationFinishedTime.Add(time.Minute)},
				}})).To(Succeed())

				By("Run cleanup procedure")
				Expect(DeleteOldServiceAccountSecrets(ctx, logger, targetClient, lastInitiationFinishedTime)).To(Succeed())

				By("Read ServiceAccounts after running cleanup procedure")
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(sa1), sa1)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(sa2), sa2)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(sa3), sa3)).To(Succeed())

				By("Performing assertions")
				Expect(targetClient.Get(ctx, client.ObjectKey{Name: "sa1secret1", Namespace: sa1.Namespace}, &corev1.Secret{})).To(BeNotFoundError())
				Expect(targetClient.Get(ctx, client.ObjectKey{Name: "sa2secret1", Namespace: sa2.Namespace}, &corev1.Secret{})).To(BeNotFoundError())
				Expect(targetClient.Get(ctx, client.ObjectKey{Name: "sa3secret1", Namespace: sa3.Namespace}, &corev1.Secret{})).To(BeNotFoundError())

				Expect(sa1.Secrets).To(BeEmpty())

				Expect(sa2.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa2-token" + suffix}))
				Expect(targetClient.Get(ctx, client.ObjectKey{Name: sa2.Secrets[0].Name, Namespace: sa2.Namespace}, &corev1.Secret{})).To(Succeed())

				Expect(sa3.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))
				Expect(sa3.Secrets).To(BeEmpty())
			})
		})
	})

})

func verifyCreatedSATokenSecret(secret *corev1.Secret, serviceAccountName string) {
	ExpectWithOffset(1, secret.Type).To(Equal(corev1.SecretTypeServiceAccountToken))
	ExpectWithOffset(1, secret.Annotations).To(HaveKeyWithValue("kubernetes.io/service-account.name", serviceAccountName))
}
