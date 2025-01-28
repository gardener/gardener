// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/test/framework"
)

// KubeconfigVerifier verifies the kubeconfig credentials rotation.
type KubeconfigVerifier struct {
	*framework.ShootCreationFramework

	oldKubeconfigData map[string][]byte
	newKubeconfigData map[string][]byte
}

// Before is called before the rotation is started.
func (v *KubeconfigVerifier) Before(ctx context.Context) {
	By("Verify old kubeconfig secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "kubeconfig")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("ca.crt", Not(BeEmpty())),
			HaveKeyWithValue("kubeconfig", Not(BeEmpty())),
		))
		v.oldKubeconfigData = secret.Data

		kubeconfig := &clientcmdv1.Config{}
		_, _, err := clientcmdlatest.Codec.Decode(secret.Data["kubeconfig"], nil, kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		Expect(kubeconfig.Clusters).To(HaveLen(1))
		Expect(kubeconfig.Clusters[0].Cluster.CertificateAuthorityData).To(Equal(secret.Data["ca.crt"]))
		Expect(kubeconfig.AuthInfos).To(HaveLen(1))
		Expect(kubeconfig.AuthInfos[0].AuthInfo.Token).NotTo(BeEmpty())
	}).Should(Succeed(), "old kubeconfig secret should be present")
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *KubeconfigVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.Kubeconfig.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *KubeconfigVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *KubeconfigVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *KubeconfigVerifier) AfterPrepared(ctx context.Context) {
	kubeconfigRotation := v.Shoot.Status.Credentials.Rotation.Kubeconfig
	Expect(kubeconfigRotation.LastCompletionTime.Time.UTC().After(kubeconfigRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Verify new kubeconfig secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "kubeconfig")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("ca.crt", Not(Equal(v.oldKubeconfigData["ca.crt"]))),
			HaveKeyWithValue("kubeconfig", Not(Equal(v.oldKubeconfigData["kubeconfig"]))),
		))
		v.newKubeconfigData = secret.Data

		kubeconfig := &clientcmdv1.Config{}
		_, _, err := clientcmdlatest.Codec.Decode(secret.Data["kubeconfig"], nil, kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		Expect(kubeconfig.Clusters).To(HaveLen(1))
		Expect(kubeconfig.Clusters[0].Cluster.CertificateAuthorityData).To(Equal(secret.Data["ca.crt"]))
		Expect(kubeconfig.AuthInfos).To(HaveLen(1))
		Expect(kubeconfig.AuthInfos[0].AuthInfo.Token).NotTo(BeEmpty())
	}).Should(Succeed(), "kubeconfig secret should have been rotated")
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *KubeconfigVerifier) ExpectCompletingStatus(_ Gomega) {
	// there is no second phase for the kubeconfig rotation
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *KubeconfigVerifier) AfterCompleted(ctx context.Context) {
	// Rotation of the kubeconfig credential (static token) as such is completed after one reconciliation
	// (there is no second phase). Hence, after completing the credentials rotation the token will be the same as after
	// preparation. We want to inspect the contained CA nevertheless, which must have changed after Completion.
	By("Verify new kubeconfig secret with new CA")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "kubeconfig")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("ca.crt", Not(Equal(v.newKubeconfigData["ca.crt"]))),
			HaveKeyWithValue("kubeconfig", Not(Equal(v.newKubeconfigData["kubeconfig"]))),
		))

		kubeconfig := &clientcmdv1.Config{}
		_, _, err := clientcmdlatest.Codec.Decode(secret.Data["kubeconfig"], nil, kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		Expect(kubeconfig.Clusters).To(HaveLen(1))
		Expect(kubeconfig.Clusters[0].Cluster.CertificateAuthorityData).To(Equal(secret.Data["ca.crt"]))
		Expect(kubeconfig.AuthInfos).To(HaveLen(1))
		Expect(kubeconfig.AuthInfos[0].AuthInfo.Token).NotTo(BeEmpty())
	}).Should(Succeed(), "kubeconfig secret should have been rotated")
}
