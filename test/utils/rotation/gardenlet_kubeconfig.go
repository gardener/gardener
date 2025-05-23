// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test"
)

const (
	gardenletDeploymentName      = "gardenlet"
	gardenletDeploymentNamespace = "garden"
)

// GardenletKubeconfigRotationVerifier verifies if a gardenlet kubeconfig rotation was successful
type GardenletKubeconfigRotationVerifier struct {
	GardenReader client.Reader
	SeedReader   client.Reader
	Seed         *gardencorev1beta1.Seed

	GardenletKubeconfigSecretName      string
	GardenletKubeconfigSecretNamespace string

	timeBeforeRotation time.Time
	oldGardenletName   string
}

// Before saves the status before the rotation
func (v *GardenletKubeconfigRotationVerifier) Before(ctx context.Context) {
	v.timeBeforeRotation = time.Now().UTC()

	Eventually(func(_ Gomega) {
		Expect(v.GardenReader.Get(ctx, client.ObjectKeyFromObject(v.Seed), v.Seed)).To(Succeed())
		v.oldGardenletName = v.Seed.Status.Gardener.Name
	}).Should(Succeed())
}

// After verifies the state after the rotation
func (v *GardenletKubeconfigRotationVerifier) After(parentCtx context.Context, expectPodRestart bool) {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	if expectPodRestart {
		By("Verify that new gardenlet pod has taken over responsibility for seed")
		CEventually(ctx, func() error {
			if err := v.GardenReader.Get(ctx, client.ObjectKeyFromObject(v.Seed), v.Seed); err != nil {
				return err
			}

			if v.Seed.Status.Gardener.Name != v.oldGardenletName {
				return nil
			}

			return fmt.Errorf("new gardenlet pod has not yet taken over responsibility for seed: %s", client.ObjectKeyFromObject(v.Seed))
		}).WithPolling(5 * time.Second).Should(Succeed())
	}

	By("Verify that gardenlet's kubeconfig secret has actually been renewed")
	CEventually(ctx, func() error {
		secret := &corev1.Secret{}
		if err := v.SeedReader.Get(ctx, client.ObjectKey{Name: v.GardenletKubeconfigSecretName, Namespace: v.GardenletKubeconfigSecretNamespace}, secret); err != nil {
			return err
		}

		kubeconfig := &clientcmdv1.Config{}
		if _, _, err := clientcmdlatest.Codec.Decode(secret.Data["kubeconfig"], nil, kubeconfig); err != nil {
			return err
		}

		clientCertificate, err := utils.DecodeCertificate(kubeconfig.AuthInfos[0].AuthInfo.ClientCertificateData)
		if err != nil {
			return err
		}

		newClientCertificateIssuedAt := clientCertificate.NotBefore.UTC()
		// The kube-controller-manager always backdates the issued certificate by 5m, see https://github.com/kubernetes/kubernetes/blob/252935368ab67f38cb252df0a961a6dcb81d20eb/pkg/controller/certificates/signer/signer.go#L197.
		// Consequently, we add these 5m so that we can assert whether the certificate was actually issued after the
		// time we recorded before the rotation was triggered.
		newClientCertificateIssuedAt = newClientCertificateIssuedAt.Add(5 * time.Minute)

		// The newClientCertificateIssuedAt time does not contain any nanoseconds, however the v.timeBeforeRotation
		// does. This was leading to failing tests in case the new client certificate was issued at the very same second
		// like the v.timeBeforeRotation, e.g. v.timeBeforeRotation = 2022-09-02 20:12:24.058418988 +0000 UTC,
		// newClientCertificateIssuedAt = 2022-09-02 20:12:24 +0000 UTC. Hence, let's round the times down to the second
		// to avoid such discrepancies. See https://github.com/gardener/gardener/issues/6618 for more details.
		newClientCertificateIssuedAt = newClientCertificateIssuedAt.Truncate(time.Second)
		timeBeforeRotation := v.timeBeforeRotation.Truncate(time.Second)

		if newClientCertificateIssuedAt.Equal(timeBeforeRotation) || newClientCertificateIssuedAt.After(timeBeforeRotation) {
			return nil
		}

		return fmt.Errorf("kubeconfig secret has not yet been renewed, timeBeforeRotation: %s, newClientCertificateIssuedAt: %s", v.timeBeforeRotation, newClientCertificateIssuedAt)
	}).WithPolling(5 * time.Second).Should(Succeed())

	By("Verify that gardenlet's deployment is updated and healthy after kubeconfig secret was renewed")
	gardenletDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: gardenletDeploymentName, Namespace: gardenletDeploymentNamespace}}
	isUpdated := health.IsDeploymentUpdated(v.SeedReader, gardenletDeployment)
	CEventually(ctx, func(g Gomega) {
		updated, err := isUpdated(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(updated).To(BeTrue())
	}).WithPolling(5 * time.Second).Should(Succeed())
}
