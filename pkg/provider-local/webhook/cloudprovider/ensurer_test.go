// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	. "github.com/gardener/gardener/pkg/provider-local/webhook/cloudprovider"
)

var _ = Describe("Ensurer", func() {
	const (
		serverHost = "https://seed.example.com:6443"
		token      = "the-workload-identity-token"
	)

	var (
		ctx     = context.Background()
		ensurer = NewEnsurer(&rest.Config{Host: serverHost}, log.Log)
	)

	decodeKubeconfig := func(raw []byte) *clientcmdv1.Config {
		obj, _, err := clientcmdlatest.Codec.Decode(raw, nil, &clientcmdv1.Config{})
		Expect(err).NotTo(HaveOccurred())
		return obj.(*clientcmdv1.Config)
	}

	secretWithProviderLabel := func(provider string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{securityv1alpha1constants.LabelWorkloadIdentityProvider: provider},
			},
			Data: map[string][]byte{securityv1alpha1constants.DataKeyToken: []byte(token)},
		}
	}

	Describe("#EnsureCloudProviderSecret", func() {
		It("should synthesize a token-based kubeconfig for a workload identity secret", func() {
			secret := secretWithProviderLabel("local")

			Expect(ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)).To(Succeed())

			Expect(secret.Data).To(HaveKey("kubeconfig"))
			kubeconfig := decodeKubeconfig(secret.Data["kubeconfig"])
			Expect(kubeconfig.Clusters).To(HaveLen(1))
			Expect(kubeconfig.Clusters[0].Cluster.Server).To(Equal(serverHost))
			Expect(kubeconfig.AuthInfos).To(HaveLen(1))
			Expect(kubeconfig.AuthInfos[0].AuthInfo.Token).To(Equal(token))
		})

		It("should not touch a secret without the workload identity provider label", func() {
			secret := &corev1.Secret{
				Data: map[string][]byte{securityv1alpha1constants.DataKeyToken: []byte(token)},
			}

			Expect(ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)).To(Succeed())
			Expect(secret.Data).NotTo(HaveKey("kubeconfig"))
		})

		It("should not touch a secret whose provider label targets another provider", func() {
			secret := secretWithProviderLabel("aws")

			Expect(ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)).To(Succeed())
			Expect(secret.Data).NotTo(HaveKey("kubeconfig"))
		})
	})
})
