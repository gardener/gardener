// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("#bootstrapCluster", func() {
	var (
		fakeClient          client.Client
		fakeDiscoveryClient *fakediscovery.FakeDiscovery
		sm                  secretsmanager.Interface

		ctx       = context.TODO()
		namespace = "garden"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeDiscoveryClient = &fakediscovery.FakeDiscovery{Fake: &testing.Fake{}}
		fakeDiscoveryClient.FakedServerVersion = &version.Info{GitVersion: "1.27.4"}
		sm = fakesecretsmanager.New(fakeClient, namespace)
	})

	It("should return an error because the garden version cannot be parsed", func() {
		fakeDiscoveryClient.FakedServerVersion.GitVersion = ""
		Expect(bootstrapCluster(ctx, fakeClient, fakeDiscoveryClient, sm)).To(MatchError(ContainSubstring("Invalid Semantic Version")))
	})

	It("should return an error because the garden version is too low", func() {
		fakeDiscoveryClient.FakedServerVersion.GitVersion = "1.26.5"
		Expect(bootstrapCluster(ctx, fakeClient, fakeDiscoveryClient, sm)).To(MatchError(ContainSubstring("the Kubernetes version of the Garden cluster must be at least 1.27")))
	})

	It("should generate a global monitoring secret because none exists yet", func() {
		Expect(bootstrapCluster(ctx, fakeClient, fakeDiscoveryClient, sm)).To(Succeed())

		secretList := &corev1.SecretList{}
		Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{"gardener.cloud/role": "global-monitoring"})).To(Succeed())
		validateGlobalMonitoringSecret(secretList)
	})

	It("should generate a global monitoring secret because secret managed by secrets-manager exists", func() {
		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "observability-ingress-0da36eb1",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "global-monitoring",
					"managed-by":          "secrets-manager",
					"manager-identity":    "controller-manager",
				},
			},
		}
		Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

		Expect(bootstrapCluster(ctx, fakeClient, fakeDiscoveryClient, sm)).To(Succeed())

		secretList := &corev1.SecretList{}
		Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{"gardener.cloud/role": "global-monitoring"})).To(Succeed())
		validateGlobalMonitoringSecret(secretList)
	})

	It("should not generate a global monitoring secret because it is managed by human operator", func() {
		customSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "self-managed-secret",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "global-monitoring",
				},
			},
		}
		Expect(fakeClient.Create(ctx, customSecret)).To(Succeed())

		Expect(bootstrapCluster(ctx, fakeClient, fakeDiscoveryClient, sm)).To(Succeed())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(customSecret), &corev1.Secret{})).To(Succeed())

		secretList := &corev1.SecretList{}
		Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
			"name":             "observability-ingress",
			"managed-by":       "secretsmanager",
			"manager-identity": "fake",
		})).To(Succeed())
		Expect(secretList.Items).To(BeEmpty())
	})
})

func validateGlobalMonitoringSecret(secretList *corev1.SecretList) {
	Expect(secretList.Items).To(HaveLen(1))
	Expect(secretList.Items[0].Name).To(HavePrefix("observability-ingress-"))
	Expect(secretList.Items[0].Labels).To(And(
		HaveKeyWithValue("name", "observability-ingress"),
		HaveKeyWithValue("managed-by", "secrets-manager"),
		HaveKeyWithValue("manager-identity", "fake"),
	))
}
