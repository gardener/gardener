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

type CAVerifier struct {
	*framework.ShootCreationFramework

	oldCACert []byte
	caBundle  []byte
	newCACert []byte

	providerLocalSecretsBefore   secretConfigNamesToSecrets
	providerLocalSecretsPrepared secretConfigNamesToSecrets
}

var managedByProviderLocalSecretsManager = client.MatchingLabels{
	"managed-by":       "secrets-manager",
	"manager-identity": "provider-local-controlplane",
}

const (
	caProviderLocalControlPlane       = "ca-provider-local-controlplane"
	caProviderLocalControlPlaneBundle = "ca-provider-local-controlplane-bundle"
	providerLocalDummyServer          = "provider-local-dummy-server"
	providerLocalDummyClient          = "provider-local-dummy-client"
	providerLocalDummyAuth            = "provider-local-dummy-auth"
)

func (v *CAVerifier) Before(ctx context.Context) {
	By("Verify old CA secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
		v.oldCACert = secret.Data["ca.crt"]

		verifyCABundleInKubeconfigSecret(ctx, g, v.GardenClient.Client(), client.ObjectKeyFromObject(v.Shoot), v.oldCACert)
	}).Should(Succeed(), "old CA cert should be synced to garden")

	By("Verify secrets of provider-local before rotation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.ShootFramework.SeedClient.Client().List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

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

func (v *CAVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

func (v *CAVerifier) AfterPrepared(ctx context.Context) {
	Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "ca rotation phase should be 'Prepared'")

	By("Verify CA bundle secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(string(secret.Data["ca.crt"])).To(ContainSubstring(string(v.oldCACert)))
		v.caBundle = secret.Data["ca.crt"]

		v.newCACert = []byte(strings.Replace(string(v.caBundle), string(v.oldCACert), "", -1))
		Expect(v.newCACert).NotTo(BeEmpty())

		verifyCABundleInKubeconfigSecret(ctx, g, v.GardenClient.Client(), client.ObjectKeyFromObject(v.Shoot), v.caBundle)
	}).Should(Succeed(), "CA bundle should be synced to garden")

	By("Verify secrets of provider-local after preparation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.ShootFramework.SeedClient.Client().List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

		v.providerLocalSecretsPrepared = groupByName(secretList.Items)
		g.Expect(v.providerLocalSecretsPrepared).To(And(
			HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(2)),
			HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
		), "CA should get rotated, but old CA and server secrets are kept")

		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(caProviderLocalControlPlane, ContainElement(v.providerLocalSecretsBefore[caProviderLocalControlPlane][0])), "old CA secret should be kept")
		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(caProviderLocalControlPlaneBundle, Not(ContainElement(v.providerLocalSecretsBefore[caProviderLocalControlPlaneBundle][0]))), "CA bundle should have changed")
		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(providerLocalDummyServer, ContainElement(v.providerLocalSecretsBefore[providerLocalDummyServer][0])), "server cert should be kept (signed with old CA)")
		g.Expect(v.providerLocalSecretsPrepared).To(HaveKeyWithValue(providerLocalDummyClient, Not(ContainElement(v.providerLocalSecretsBefore[providerLocalDummyServer][0]))), "client cert should have changed (signed with new CA)")
	}).Should(Succeed())
}

func (v *CAVerifier) ExpectCompletingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleting))
}

func (v *CAVerifier) AfterCompleted(ctx context.Context) {
	// After completing CA rotation, it might take some time until all conditions are healthy again.
	// Hence, we cannot expect the completion time to be relatively close to time.Now(). Instead we only expect, that
	// the completion time is after the last initiation time.
	caRotation := v.Shoot.Status.Credentials.Rotation.CertificateAuthorities
	Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(caRotation.LastCompletionTime.Time.UTC().After(caRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Verify new CA secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", v.newCACert))

		verifyCABundleInKubeconfigSecret(ctx, g, v.GardenClient.Client(), client.ObjectKeyFromObject(v.Shoot), v.newCACert)
	}).Should(Succeed(), "new CA cert should be synced to garden")

	By("Verify secrets of provider-local after completion")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.ShootFramework.SeedClient.Client().List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

		grouped := groupByName(secretList.Items)
		g.Expect(grouped).To(And(
			HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(1)),
			HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
			HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
		), "old CA secret should get cleaned up")

		g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlane, ContainElement(v.providerLocalSecretsPrepared[caProviderLocalControlPlane][1])), "new CA secret should be kept")
		g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlaneBundle, Not(ContainElement(v.providerLocalSecretsPrepared[caProviderLocalControlPlaneBundle][0]))), "CA bundle should have changed")
		g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyServer, Not(ContainElement(v.providerLocalSecretsPrepared[providerLocalDummyServer][0]))), "server cert should have changed (signed with new CA)")
		g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyClient, ContainElement(v.providerLocalSecretsPrepared[providerLocalDummyClient][0])), "client cert sould be kept (already signed with new CA)")
	}).Should(Succeed())
}

func verifyCABundleInKubeconfigSecret(ctx context.Context, g Gomega, c client.Reader, shootKey client.ObjectKey, expectedBundle []byte) {
	secret := &corev1.Secret{}
	shootKey.Name = gutil.ComputeShootProjectSecretName(shootKey.Name, "kubeconfig")
	g.Expect(c.Get(ctx, shootKey, secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", expectedBundle))
}
