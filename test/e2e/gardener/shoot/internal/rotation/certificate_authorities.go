// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/rotation"
)

// CAVerifier verifies the certificate authorities rotation.
type CAVerifier struct {
	*framework.ShootCreationFramework

	oldCACert []byte
	caBundle  []byte
	newCACert []byte

	gardenletSecretsBefore       rotation.SecretConfigNamesToSecrets
	gardenletSecretsPrepared     rotation.SecretConfigNamesToSecrets
	gardenletSecretsCompleted    rotation.SecretConfigNamesToSecrets
	providerLocalSecretsBefore   rotation.SecretConfigNamesToSecrets
	providerLocalSecretsPrepared rotation.SecretConfigNamesToSecrets
}

var managedByProviderLocalSecretsManager = client.MatchingLabels{
	"managed-by":       "secrets-manager",
	"manager-identity": "provider-local-controlplane",
}

const (
	caCluster       = "ca"
	caClient        = "ca-client"
	caETCD          = "ca-etcd"
	caETCDPeer      = "ca-etcd-peer"
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

	By("Verify CA secrets of gardenlet before rotation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), ManagedByGardenletSecretsManager)).To(Succeed())

		allGardenletCAs := getAllGardenletCAs(v1beta1helper.IsWorkerless(v.Shoot))
		grouped := rotation.GroupByName(secretList.Items)
		for _, ca := range allGardenletCAs {
			bundle := ca + "-bundle"
			g.Expect(grouped[ca]).To(HaveLen(1), ca+" secret should get created, but not rotated yet")
			g.Expect(grouped[bundle]).To(HaveLen(1), ca+" bundle secret should get created, but not rotated yet")
		}
		v.gardenletSecretsBefore = grouped
	}).Should(Succeed())

	By("Verify old CA config map in garden cluster")
	Eventually(func(g Gomega) {
		configMap := &corev1.ConfigMap{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ca-cluster")}, configMap)).To(Succeed())
		g.Expect([]byte(configMap.Data["ca.crt"])).To(And(
			Not(BeEmpty()),
			Equal(v.gardenletSecretsBefore[caCluster+"-bundle"][0].Data["bundle.crt"]),
		), "ca-cluster config map in garden should contain the same bundle as ca-bundle secret on seed")
		v.oldCACert = []byte(configMap.Data["ca.crt"])
	}).Should(Succeed(), "old CA cert should be synced to garden")

	By("Verify old CA secret in garden cluster")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(secret.Data["ca.crt"]).To(And(
			Not(BeEmpty()),
			Equal(v.gardenletSecretsBefore[caCluster+"-bundle"][0].Data["bundle.crt"]),
		), "ca-cluster secret in garden should contain the same bundle as ca-bundle secret on seed")
		oldCACert := secret.Data["ca.crt"]
		Expect(oldCACert).To(Equal(v.oldCACert), "ca-cluster secret in garden should contain the same bundle as ca-cluster config map in garden")
	}).Should(Succeed(), "old CA cert should be synced to garden")

	if !v1beta1helper.IsWorkerless(v.Shoot) {
		By("Verify secrets of provider-local before rotation")
		Eventually(func(g Gomega) {
			secretList := &corev1.SecretList{}
			g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

			grouped := rotation.GroupByName(secretList.Items)
			g.Expect(grouped[caProviderLocalControlPlane]).To(HaveLen(1), "CA secret should get created, but not rotated yet")
			g.Expect(grouped[caProviderLocalControlPlaneBundle]).To(HaveLen(1), "CA bundle secret should get created, but not rotated yet")
			g.Expect(grouped[providerLocalDummyServer]).To(HaveLen(1))
			g.Expect(grouped[providerLocalDummyClient]).To(HaveLen(1))
			g.Expect(grouped[providerLocalDummyAuth]).To(HaveLen(1))
			v.providerLocalSecretsBefore = grouped
		}).Should(Succeed())
	}
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *CAVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
	g.Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
	g.Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime).To(BeNil())
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *CAVerifier) ExpectPreparingWithoutWorkersRolloutStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationPreparingWithoutWorkersRollout))
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
	g.Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
	g.Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime).To(BeNil())
}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *CAVerifier) ExpectWaitingForWorkersRolloutStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
	g.Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime).NotTo(BeNil())
	g.Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
	g.Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime).To(BeNil())
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *CAVerifier) AfterPrepared(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()
	Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "ca rotation phase should be 'Prepared'")
	Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).NotTo(BeNil())
	Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.After(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time)).To(BeTrue())

	By("Verify CA secrets of gardenlet after preparation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), ManagedByGardenletSecretsManager)).To(Succeed())

		allGardenletCAs := getAllGardenletCAs(v1beta1helper.IsWorkerless(v.Shoot))
		grouped := rotation.GroupByName(secretList.Items)
		for _, ca := range allGardenletCAs {
			bundle := ca + "-bundle"
			g.Expect(grouped[ca]).To(HaveLen(2), ca+" secret should get rotated, but old CA is kept")
			g.Expect(grouped[bundle]).To(HaveLen(1), ca+" bundle secret should have changed")
			g.Expect(grouped[ca]).To(ContainElement(v.gardenletSecretsBefore[ca][0]), "old "+ca+" secret should be kept")
			g.Expect(grouped[bundle]).To(Not(ContainElement(v.gardenletSecretsBefore[bundle][0])), "old "+ca+" bundle should get cleaned up")
		}
		v.gardenletSecretsPrepared = grouped
	}).Should(Succeed())

	By("Verify CA bundle config map in garden cluster")
	Eventually(func(g Gomega) {
		configMap := &corev1.ConfigMap{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ca-cluster")}, configMap)).To(Succeed())
		g.Expect([]byte(configMap.Data["ca.crt"])).To(And(
			Not(BeEmpty()),
			Equal(v.gardenletSecretsPrepared[caCluster+"-bundle"][0].Data["bundle.crt"]),
		), "ca-cluster config map in garden should contain the same bundle as ca-bundle secret on seed")
		g.Expect(configMap.Data["ca.crt"]).To(ContainSubstring(string(v.oldCACert)), "CA bundle should contain the old CA cert")
		v.caBundle = []byte(configMap.Data["ca.crt"])

		v.newCACert = []byte(strings.Replace(string(v.caBundle), string(v.oldCACert), "", -1))
		Expect(v.newCACert).NotTo(BeEmpty())
	}).Should(Succeed(), "CA bundle should be synced to garden")

	By("Verify CA bundle secret in garden cluster")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(secret.Data["ca.crt"]).To(And(
			Not(BeEmpty()),
			Equal(v.gardenletSecretsPrepared[caCluster+"-bundle"][0].Data["bundle.crt"]),
		), "ca-cluster secret in garden should contain the same bundle as ca-bundle secret on seed")
		g.Expect(string(secret.Data["ca.crt"])).To(ContainSubstring(string(v.oldCACert)), "CA bundle should contain the old CA cert")
		caBundle := secret.Data["ca.crt"]
		Expect(caBundle).To(Equal(v.caBundle), "ca-cluster secret in garden should contain the same bundle as ca-cluster config map in garden")

		newCACert := []byte(strings.Replace(string(caBundle), string(v.oldCACert), "", -1))
		Expect(newCACert).To(Equal(v.newCACert), "new CA bundle from secret in garden should match new CA bundle from config map in garden")
	}).Should(Succeed(), "CA bundle should be synced to garden")

	if !v1beta1helper.IsWorkerless(v.Shoot) {
		By("Verify secrets of provider-local after preparation")
		Eventually(func(g Gomega) {
			secretList := &corev1.SecretList{}
			g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

			grouped := rotation.GroupByName(secretList.Items)
			g.Expect(grouped[caProviderLocalControlPlane]).To(HaveLen(2), "CA secret should get rotated, but old CA is kept")
			g.Expect(grouped[caProviderLocalControlPlaneBundle]).To(HaveLen(1), "CA bundle secret should have changed")
			g.Expect(grouped[providerLocalDummyServer]).To(HaveLen(1))
			g.Expect(grouped[providerLocalDummyClient]).To(HaveLen(1))
			g.Expect(grouped[providerLocalDummyAuth]).To(HaveLen(1))

			g.Expect(grouped[caProviderLocalControlPlane]).To(ContainElement(v.providerLocalSecretsBefore[caProviderLocalControlPlane][0]), "old CA secret should be kept")
			g.Expect(grouped[caProviderLocalControlPlaneBundle]).To(Not(ContainElement(v.providerLocalSecretsBefore[caProviderLocalControlPlaneBundle][0])), "old CA bundle should get cleaned up")
			g.Expect(grouped[providerLocalDummyServer]).To(ContainElement(v.providerLocalSecretsBefore[providerLocalDummyServer][0]), "server cert should be kept (signed with old CA)")
			g.Expect(grouped[providerLocalDummyClient]).To(Not(ContainElement(v.providerLocalSecretsBefore[providerLocalDummyServer][0])), "client cert should have changed (signed with new CA)")
			v.providerLocalSecretsPrepared = grouped
		}).Should(Succeed())
	}
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *CAVerifier) ExpectCompletingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleting))
	Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime).NotTo(BeNil())
	Expect(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime.Time.Equal(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.Time) ||
		v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime.After(v.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.Time)).To(BeTrue())
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *CAVerifier) AfterCompleted(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	caRotation := v.Shoot.Status.Credentials.Rotation.CertificateAuthorities
	Expect(v1beta1helper.GetShootCARotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(caRotation.LastCompletionTime.Time.UTC().After(caRotation.LastInitiationTime.Time.UTC())).To(BeTrue())
	Expect(caRotation.LastInitiationFinishedTime).To(BeNil())
	Expect(caRotation.LastCompletionTriggeredTime).To(BeNil())

	By("Verify CA secrets of gardenlet after completion")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), ManagedByGardenletSecretsManager)).To(Succeed())

		allGardenletCAs := getAllGardenletCAs(v1beta1helper.IsWorkerless(v.Shoot))
		grouped := rotation.GroupByName(secretList.Items)
		for _, ca := range allGardenletCAs {
			bundle := ca + "-bundle"
			g.Expect(grouped[ca]).To(HaveLen(1), "old "+ca+" secret should get cleaned up")
			g.Expect(grouped[bundle]).To(HaveLen(1), ca+" bundle secret should have changed")
			g.Expect(grouped[ca]).To(ContainElement(v.gardenletSecretsPrepared[ca][1]), "new "+ca+" secret should be kept")
			g.Expect(grouped[bundle]).To(Not(ContainElement(v.gardenletSecretsPrepared[bundle][0])), "combined "+ca+" bundle should get cleaned up")
		}
		v.gardenletSecretsCompleted = grouped
	}).Should(Succeed())

	By("Verify new CA config map in garden cluster")
	Eventually(func(g Gomega) {
		configMap := &corev1.ConfigMap{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ca-cluster")}, configMap)).To(Succeed())
		g.Expect([]byte(configMap.Data["ca.crt"])).To(And(
			Not(BeEmpty()),
			Equal(v.gardenletSecretsCompleted[caCluster+"-bundle"][0].Data["bundle.crt"]),
		), "ca-cluster config map in garden should contain the same bundle as ca-bundle secret on seed")
		g.Expect([]byte(configMap.Data["ca.crt"])).To(Equal(v.newCACert), "new CA bundle should only contain new CA cert")
	}).Should(Succeed(), "new CA cert should be synced to garden")

	By("Verify new CA secret in garden cluster")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ca-cluster")}, secret)).To(Succeed())
		g.Expect(secret.Data["ca.crt"]).To(And(
			Not(BeEmpty()),
			Equal(v.gardenletSecretsCompleted[caCluster+"-bundle"][0].Data["bundle.crt"]),
		), "ca-cluster secret in garden should contain the same bundle as ca-bundle secret on seed")
		g.Expect(secret.Data["ca.crt"]).To(Equal(v.newCACert), "new CA bundle should only contain new CA cert")
	}).Should(Succeed(), "new CA cert should be synced to garden")

	if !v1beta1helper.IsWorkerless(v.Shoot) {
		By("Verify secrets of provider-local after completion")
		Eventually(func(g Gomega) {
			secretList := &corev1.SecretList{}
			g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByProviderLocalSecretsManager)).To(Succeed())

			grouped := rotation.GroupByName(secretList.Items)
			g.Expect(grouped[caProviderLocalControlPlane]).To(HaveLen(1), "old CA secret should get cleaned up")
			g.Expect(grouped[caProviderLocalControlPlaneBundle]).To(HaveLen(1), "CA bundle secret should have changed")
			g.Expect(grouped[providerLocalDummyServer]).To(HaveLen(1))
			g.Expect(grouped[providerLocalDummyClient]).To(HaveLen(1))
			g.Expect(grouped[providerLocalDummyAuth]).To(HaveLen(1))

			g.Expect(grouped[caProviderLocalControlPlane]).To(ContainElement(v.providerLocalSecretsPrepared[caProviderLocalControlPlane][1]), "new CA secret should be kept")
			g.Expect(grouped[caProviderLocalControlPlaneBundle]).To(Not(ContainElement(v.providerLocalSecretsPrepared[caProviderLocalControlPlaneBundle][0])), "combined CA bundle should get cleaned up")
			g.Expect(grouped[providerLocalDummyServer]).To(Not(ContainElement(v.providerLocalSecretsPrepared[providerLocalDummyServer][0])), "server cert should have changed (signed with new CA)")
			g.Expect(grouped[providerLocalDummyClient]).To(ContainElement(v.providerLocalSecretsPrepared[providerLocalDummyClient][0]), "client cert should be kept (already signed with new CA)")
		}).Should(Succeed())
	}
}

func getAllGardenletCAs(isWorkerless bool) []string {
	allGardenletCAs := []string{caCluster, caClient, caETCD, caETCDPeer, caFrontProxy}

	if !isWorkerless {
		allGardenletCAs = append(allGardenletCAs, caKubelet, caMetricsServer, caVPN)
	}

	return allGardenletCAs
}
