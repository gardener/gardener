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

package shoot

import (
	"context"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Shoot Tests", Label("Shoot"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = defaultShoot("rotate-ca-")

	It("Create Shoot, Rotate CA and Delete Shoot", Label("ca-rotation"), func() {
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()

		By("Create Shoot")
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Verify old CA secret")
		var oldCACert []byte
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{}
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(f.Shoot.Name, gutil.ShootProjectSecretSuffixCACluster)}, secret)).To(Succeed())
			g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
			oldCACert = secret.Data["ca.crt"]

			verifyCABundleInKubeconfigSecret(ctx, g, f.GardenClient.Client(), client.ObjectKeyFromObject(f.Shoot), oldCACert)
		}).Should(Succeed(), "old CA cert should be synced to garden")

		By("Verify secrets of provider-local before rotation")
		seedClient := f.ShootFramework.SeedClient.Client()
		var providerLocalSecretsBefore secretConfigNamesToSecrets
		Eventually(func(g Gomega) {
			providerLocalSecretsBefore = verifyProviderLocalSecretsBefore(ctx, g, seedClient, f.Shoot.Status.TechnicalID)
		}).Should(Succeed())

		By("Start CA rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		patch := client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCAStart)
		Eventually(func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			return helper.GetShootCARotationPhase(f.Shoot.Status.Credentials) == gardencorev1beta1.RotationPreparing &&
				!metav1.HasAnnotation(f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation) &&
				time.Now().UTC().Sub(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.UTC()) <= time.Minute
		}).Should(BeTrue())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func() error {
			return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
		}).Should(Succeed())
		Expect(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "ca rotation phase should be 'Prepared'")

		By("Verify CA bundle secret")
		var (
			caBundle  []byte
			newCACert []byte
		)

		Eventually(func(g Gomega) {
			secret := &corev1.Secret{}
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(f.Shoot.Name, gutil.ShootProjectSecretSuffixCACluster)}, secret)).To(Succeed())
			g.Expect(string(secret.Data["ca.crt"])).To(ContainSubstring(string(oldCACert)))
			caBundle = secret.Data["ca.crt"]
			newCACert = []byte(strings.Replace(string(caBundle), string(oldCACert), "", -1))

			verifyCABundleInKubeconfigSecret(ctx, g, f.GardenClient.Client(), client.ObjectKeyFromObject(f.Shoot), caBundle)
		}).Should(Succeed(), "CA bundle should be synced to garden")

		By("Verify secrets of provider-local after preparation")
		var providerLocalSecretsPrepared secretConfigNamesToSecrets
		Eventually(func(g Gomega) {
			providerLocalSecretsPrepared = verifyProviderLocalSecretsPrepared(ctx, g, seedClient, f.Shoot.Status.TechnicalID, providerLocalSecretsBefore)
		}).Should(Succeed())

		By("Complete CA rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		patch = client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCAComplete)
		Eventually(func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			return helper.GetShootCARotationPhase(f.Shoot.Status.Credentials) == gardencorev1beta1.RotationCompleting &&
				!metav1.HasAnnotation(f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation)
		}).Should(BeTrue())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())

			// After completing CA rotation, it might take some time until all conditions are healthy again.
			// Hence, we cannot expect the completion time to be relatively close to time.Now(). Instead we only expect, that
			// the completion time is after the last initiation time.
			caRotation := f.Shoot.Status.Credentials.Rotation.CertificateAuthorities
			return helper.GetShootCARotationPhase(f.Shoot.Status.Credentials) == gardencorev1beta1.RotationCompleted &&
				caRotation.LastCompletionTime.Time.UTC().After(caRotation.LastInitiationTime.Time.UTC())
		}).Should(BeTrue())

		By("Verify new CA secret")
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{}
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(f.Shoot.Name, gutil.ShootProjectSecretSuffixCACluster)}, secret)).To(Succeed())
			g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", newCACert))

			verifyCABundleInKubeconfigSecret(ctx, g, f.GardenClient.Client(), client.ObjectKeyFromObject(f.Shoot), newCACert)
		}).Should(Succeed(), "new CA cert should be synced to garden")

		By("Verify secrets of provider-local after completion")
		Eventually(func(g Gomega) {
			verifyProviderLocalSecretsCompleted(ctx, g, seedClient, f.Shoot.Status.TechnicalID, providerLocalSecretsPrepared)
		}).Should(Succeed())

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

func verifyCABundleInKubeconfigSecret(ctx context.Context, g Gomega, c client.Reader, shootKey client.ObjectKey, expectedBundle []byte) {
	secret := &corev1.Secret{}
	shootKey.Name = gutil.ComputeShootProjectSecretName(shootKey.Name, gutil.ShootProjectSecretSuffixKubeconfig)
	g.Expect(c.Get(ctx, shootKey, secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", expectedBundle))
}

type secretConfigNamesToSecrets map[string][]corev1.Secret

func groupByName(allSecrets []corev1.Secret) secretConfigNamesToSecrets {
	grouped := make(secretConfigNamesToSecrets)
	for _, secret := range allSecrets {
		grouped[secret.Labels["name"]] = append(grouped[secret.Labels["name"]], secret)
	}

	// sort by age
	for _, secrets := range grouped {
		sort.Slice(secrets, func(i, j int) bool {
			return secrets[i].CreationTimestamp.Before(&secrets[j].CreationTimestamp)
		})
	}
	return grouped
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

func verifyProviderLocalSecretsBefore(ctx context.Context, g Gomega, c client.Reader, namespace string) secretConfigNamesToSecrets {
	secretList := &corev1.SecretList{}
	g.Expect(c.List(ctx, secretList, client.InNamespace(namespace), managedByProviderLocalSecretsManager)).To(Succeed())

	grouped := groupByName(secretList.Items)
	g.Expect(grouped).To(And(
		HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(1)),
		HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
	), "all secrets should get created, but not rotated yet")

	return grouped
}

func verifyProviderLocalSecretsPrepared(ctx context.Context, g Gomega, c client.Reader, namespace string, secretsBefore secretConfigNamesToSecrets) secretConfigNamesToSecrets {
	secretList := &corev1.SecretList{}
	g.Expect(c.List(ctx, secretList, client.InNamespace(namespace), managedByProviderLocalSecretsManager)).To(Succeed())

	grouped := groupByName(secretList.Items)
	g.Expect(grouped).To(And(
		HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(2)),
		HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
	), "CA should get rotated, but old CA and server secrets are kept")

	g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlane, ContainElement(secretsBefore[caProviderLocalControlPlane][0])), "old CA secret should be kept")
	g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlaneBundle, Not(ContainElement(secretsBefore[caProviderLocalControlPlaneBundle][0]))), "CA bundle should have changed")
	g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyServer, ContainElement(secretsBefore[providerLocalDummyServer][0])), "server cert should be kept (signed with old CA)")
	g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyClient, Not(ContainElement(secretsBefore[providerLocalDummyServer][0]))), "client cert should have changed (signed with new CA)")

	return grouped
}

func verifyProviderLocalSecretsCompleted(ctx context.Context, g Gomega, c client.Reader, namespace string, secretsPrepared secretConfigNamesToSecrets) {
	secretList := &corev1.SecretList{}
	g.Expect(c.List(ctx, secretList, client.InNamespace(namespace), managedByProviderLocalSecretsManager)).To(Succeed())

	grouped := groupByName(secretList.Items)
	g.Expect(grouped).To(And(
		HaveKeyWithValue(caProviderLocalControlPlane, HaveLen(1)),
		HaveKeyWithValue(caProviderLocalControlPlaneBundle, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyServer, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyClient, HaveLen(1)),
		HaveKeyWithValue(providerLocalDummyAuth, HaveLen(1)),
	), "old CA secret should get cleaned up")

	g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlane, ContainElement(secretsPrepared[caProviderLocalControlPlane][1])), "new CA secret should be kept")
	g.Expect(grouped).To(HaveKeyWithValue(caProviderLocalControlPlaneBundle, Not(ContainElement(secretsPrepared[caProviderLocalControlPlaneBundle][0]))), "CA bundle should have changed")
	g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyServer, Not(ContainElement(secretsPrepared[providerLocalDummyServer][0]))), "server cert should have changed (signed with new CA)")
	g.Expect(grouped).To(HaveKeyWithValue(providerLocalDummyClient, ContainElement(secretsPrepared[providerLocalDummyClient][0])), "client cert sould be kept (already signed with new CA)")
}
