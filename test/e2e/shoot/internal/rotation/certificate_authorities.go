// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package rotation

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/test/framework"
)

// CAVerifier verifies the certificate authorities rotation.
type CAVerifier struct {
	*framework.ShootCreationFramework

	oldCACert []byte
	caBundle  []byte
	newCACert []byte

	gardenletSecretsBefore       secretConfigNamesToSecrets
	gardenletSecretsPrepared     secretConfigNamesToSecrets
	providerLocalSecretsBefore   secretConfigNamesToSecrets
	providerLocalSecretsPrepared secretConfigNamesToSecrets
}

var managedByProviderLocalSecretsManager = client.MatchingLabels{
	"managed-by":       "secrets-manager",
	"manager-identity": "provider-local-controlplane",
}

var allGardenletCAs = []string{
	caCluster,
	caClient,
	caETCD,
	caFrontProxy,
	caKubelet,
	caMetricsServer,
	caVPN,
}

const (
	caCluster       = "ca"
	caClient        = "ca-client"
	caETCD          = "ca-etcd"
	caFrontProxy    = "ca-front-proxy"
	caKubelet       = "ca-kubelet"
	caMetricsServer = "ca-metrics-server"
	caVPN           = "ca-vpn"

	caProviderLocalControlPlane       = "ca-provider-local-controlplane"
	caProviderLocalControlPlaneBundle = "ca-provider-local-controlplane-bundle"
	providerLocalDummyServer          = "provider-local-dummy-server"
	providerLocalDummyClient          = "provider-local-dummy-client"
	providerLocalDummyAuth            = "provider-local-dummy-auth"
)

// Before is called before the rotation is started.
func (v *CAVerifier) Before(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	By("Verify old CA secret in garden cluster")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
		v.oldCACert = secret.Data["ca.crt"]

		verifyCABundleInKubeconfigSecret(ctx, g, v.GardenClient.Client(), client.ObjectKeyFromObject(v.Shoot), v.oldCACert)
	}).Should(Succeed(), "old CA cert should be synced to garden")

	By("Verify CA secrets of gardenlet before rotation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		v.gardenletSecretsBefore = groupByName(secretList.Items)
		for _, ca := range allGardenletCAs {
			bundle := ca + "-bundle"
			g.Expect(v.gardenletSecretsBefore).To(HaveKeyWithValue(ca, HaveLen(1)), ca+" secret should get created, but not rotated yet")
			g.Expect(v.gardenletSecretsBefore).To(HaveKeyWithValue(bundle, HaveLen(1)), ca+" bundle secret should get created, but not rotated yet")
		}
	}).Should(Succeed())

	By("Verify secrets of provider-local before rotation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

		v.providerLocalSecretsBefore = groupByName(secretList.Items)
		g.Expect(v.providerLocalSecretsBefore).To(And(
			HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(1)),
			HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
		), "all secrets should get created, but not rotated yet")
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *CAVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *CAVerifier) AfterPrepared(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()
	Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "ca rotation phase should be 'Prepared'")

	By("Verify CA bundle secret in garden cluster")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(string(secret.Data["ca.crt"])).To(ContainSubstring(string(v.oldCACert)))
		v.caBundle = secret.Data["ca.crt"]

		v.newCACert = []byte(strings.Replace(string(v.caBundle), string(v.oldCACert), "", -1))
		Expect(v.newCACert).NotTo(BeEmpty())

		verifyCABundleInKubeconfigSecret(ctx, g, v.GardenClient.Client(), client.ObjectKeyFromObject(v.Shoot), v.caBundle)
	}).Should(Succeed(), "CA bundle should be synced to garden")

	By("Verify CA secrets of gardenlet after preparation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		v.gardenletSecretsPrepared = groupByName(secretList.Items)
		for _, ca := range allGardenletCAs {
			bundle := ca + "-bundle"
			g.Expect(v.gardenletSecretsPrepared).To(HaveKeyWithValue(ca, HaveLen(2)), ca+" secret should get rotated, but old CA is kept")
			g.Expect(v.gardenletSecretsPrepared).To(HaveKeyWithValue(bundle, HaveLen(1)), ca+" bundle secret should have changed")
			g.Expect(v.gardenletSecretsPrepared).To(HaveKeyWithValue(ca, ContainElement(v.gardenletSecretsBefore[ca][0])), "old "+ca+" secret should be kept")
			g.Expect(v.gardenletSecretsPrepared).To(HaveKeyWithValue(bundle, Not(ContainElement(v.gardenletSecretsBefore[bundle][0]))), "old "+ca+" bundle should get cleaned up")
		}
	}).Should(Succeed())

	By("Verify secrets of provider-local after preparation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

		v.providerLocalSecretsPrepared = groupByName(secretList.Items)
		g.Expect(v.providerLocalSecretsPrepared).To(And(
			HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(2)),
			HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
		), "CA should get rotated, but old CA and server secrets are kept")

		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(caProviderLocalControlPlane, ContainElement(v.providerLocalSecretsBefore[caProviderLocalControlPlane][0])), "old CA secret should be kept")
		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(caProviderLocalControlPlaneBundle, Not(ContainElement(v.providerLocalSecretsBefore[caProviderLocalControlPlaneBundle][0]))), "old CA bundle should get cleaned up")
		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(providerLocalDummyServer, ContainElement(v.providerLocalSecretsBefore[providerLocalDummyServer][0])), "server cert should be kept (signed with old CA)")
		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(providerLocalDummyClient, Not(ContainElement(v.providerLocalSecretsBefore[providerLocalDummyServer][0]))), "client cert should have changed (signed with new CA)")
	}).Should(Succeed())
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *CAVerifier) ExpectCompletingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleting))
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *CAVerifier) AfterCompleted(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	caRotation := v.Shoot.Status.Credentials.Rotation.CertificateAuthorities
	Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(caRotation.LastCompletionTime.Time.UTC().After(caRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Verify new CA secret in garden cluster")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", v.newCACert))

		verifyCABundleInKubeconfigSecret(ctx, g, v.GardenClient.Client(), client.ObjectKeyFromObject(v.Shoot), v.newCACert)
	}).Should(Succeed(), "new CA cert should be synced to garden")

	By("Verify CA secrets of gardenlet after completion")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

		grouped := groupByName(secretList.Items)
		for _, ca := range allGardenletCAs {
			bundle := ca + "-bundle"
			g.Expect(grouped).To(HaveKeyWithValue(ca, HaveLen(1)), "old "+ca+" secret should get cleaned up")
			g.Expect(grouped).To(HaveKeyWithValue(bundle, HaveLen(1)), ca+" bundle secret should have changed")
			g.Expect(grouped).To(HaveKeyWithValue(ca, ContainElement(v.gardenletSecretsPrepared[ca][1])), "new "+ca+" secret should be kept")
			g.Expect(grouped).To(HaveKeyWithValue(bundle, Not(ContainElement(v.gardenletSecretsPrepared[bundle][0]))), "combined "+ca+" bundle should get cleaned up")
		}
	}).Should(Succeed())

	By("Verify secrets of provider-local after completion")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

		grouped := groupByName(secretList.Items)
		g.Expect(grouped).To(And(
			HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(1)),
			HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
		), "old CA secret should get cleaned up")

		g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlane, ContainElement(v.providerLocalSecretsPrepared[caProviderLocalControlPlane][1])), "new CA secret should be kept")
		g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlaneBundle, Not(ContainElement(v.providerLocalSecretsPrepared[caProviderLocalControlPlaneBundle][0]))), "combined CA bundle should get cleaned up")
		g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyServer, Not(ContainElement(v.providerLocalSecretsPrepared[providerLocalDummyServer][0]))), "server cert should have changed (signed with new CA)")
		g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyClient, ContainElement(v.providerLocalSecretsPrepared[providerLocalDummyClient][0])), "client cert should be kept (already signed with new CA)")
	}).Should(Succeed())
}

func verifyCABundleInKubeconfigSecret(ctx context.Context, g Gomega, c client.Reader, shootKey client.ObjectKey, expectedBundle []byte) {
	secret := &corev1.Secret{}
	shootKey.Name = gutil.ComputeShootProjectSecretName(shootKey.Name, "kubeconfig")
	g.Expect(c.Get(ctx, shootKey, secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", expectedBundle))
}
