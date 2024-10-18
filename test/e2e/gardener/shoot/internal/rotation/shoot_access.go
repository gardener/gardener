// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
)

type clients struct {
	adminKubeconfig, clientCert, serviceAccountDynamic, serviceAccountStatic kubernetes.Interface
}

// ShootAccessVerifier uses the various access methods to access the Shoot.
type ShootAccessVerifier struct {
	*framework.ShootCreationFramework

	clientsBefore, clientsPrepared, clientsAfter clients
}

// Before is called before the rotation is started.
func (v *ShootAccessVerifier) Before(ctx context.Context) {
	By("Use admin kubeconfig with old CA to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.adminKubeconfig = shootClient
	}).Should(Succeed())

	By("Request new client certificate and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsBefore.adminKubeconfig, "e2e-rotate-csr-before")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.clientCert = shootClient
	}).Should(Succeed())

	By("Request new dynamic token for a ServiceAccount and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsBefore.adminKubeconfig, "e2e-rotate-sa-dynamic-before")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.serviceAccountDynamic = shootClient
	}).Should(Succeed())

	By("Request new static token for a ServiceAccount and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, v.clientsBefore.adminKubeconfig, "e2e-rotate-sa-static-before")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.serviceAccountStatic = shootClient
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ShootAccessVerifier) ExpectPreparingStatus(_ Gomega) {}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *ShootAccessVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *ShootAccessVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ShootAccessVerifier) AfterPrepared(ctx context.Context) {
	By("Use admin kubeconfig with old CA to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsBefore.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use client certificate from before rotation to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use dynamic ServiceAccount token from before rotation to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use static ServiceAccount token from before rotation to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsBefore.serviceAccountStatic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use admin kubeconfig with CA bundle to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.adminKubeconfig = shootClient
	}).Should(Succeed())

	By("Request new client certificate and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsPrepared.adminKubeconfig, "e2e-rotate-csr-prepared")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.clientCert = shootClient
	}).Should(Succeed())

	By("Request new dynamic token for a ServiceAccount and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsPrepared.adminKubeconfig, "e2e-rotate-sa-dynamic-prepared")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.serviceAccountDynamic = shootClient
	}).Should(Succeed())

	By("Request new static token for a ServiceAccount and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, v.clientsPrepared.adminKubeconfig, "e2e-rotate-sa-static-prepared")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.serviceAccountStatic = shootClient
	}).Should(Succeed())
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ShootAccessVerifier) ExpectCompletingStatus(_ Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ShootAccessVerifier) AfterCompleted(ctx context.Context) {
	By("Use admin kubeconfig with old CA to access shoot")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Use client certificate from before rotation to access shoot")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Use dynamic ServiceAccount token from before rotation to access shoot")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Use static ServiceAccount token from before rotation to access shoot")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.serviceAccountStatic.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Use admin kubeconfig with CA bundle to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use client certificate from after preparation to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use dynamic ServiceAccount token from after preparation to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use static ServiceAccount token from after preparation to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.serviceAccountStatic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use admin kubeconfig with new CA to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsAfter.adminKubeconfig = shootClient
	}).Should(Succeed())

	By("Request new client certificate and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsAfter.adminKubeconfig, "e2e-rotate-csr-after")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsAfter.clientCert = shootClient
	}).Should(Succeed())

	By("Request new dynamic token for a ServiceAccount and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsAfter.adminKubeconfig, "e2e-rotate-sa-dynamic-after")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsAfter.serviceAccountDynamic = shootClient
	}).Should(Succeed())

	By("Request new static token for a ServiceAccount and using it to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, v.clientsAfter.adminKubeconfig, "e2e-rotate-sa-static-after")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsAfter.serviceAccountStatic = shootClient
	}).Should(Succeed())
}

// Cleanup is passed to ginkgo.DeferCleanup.
func (v *ShootAccessVerifier) Cleanup(ctx context.Context) {
	if v.Config.GardenerConfig.ExistingShootName == "" {
		// we only have to clean up if we are using an existing shoot, otherwise the shoot will be deleted
		return
	}

	// figure out the right shoot client to use, depending on how far the test was executed
	shootClient := v.clientsBefore.adminKubeconfig
	if shootClient == nil {
		// shoot was never successfully created or accessed, nothing to delete
		return
	}
	if v.clientsPrepared.adminKubeconfig != nil {
		shootClient = v.clientsPrepared.adminKubeconfig
	}
	if v.clientsAfter.adminKubeconfig != nil {
		shootClient = v.clientsAfter.adminKubeconfig
	}

	By("Clean up objects in shoot from client certificate access")
	Eventually(func(g Gomega) {
		g.Expect(access.CleanupObjectsFromCSRAccess(ctx, shootClient)).To(Succeed())
	}).Should(Succeed())

	By("Clean up objects in shoot from dynamic ServiceAccount token access")
	Eventually(func(g Gomega) {
		g.Expect(access.CleanupObjectsFromDynamicServiceAccountTokenAccess(ctx, shootClient)).To(Succeed())
	}).Should(Succeed())

	By("Clean up objects in shoot from static ServiceAccount token access")
	Eventually(func(g Gomega) {
		g.Expect(access.CleanupObjectsFromStaticServiceAccountTokenAccess(ctx, shootClient)).To(Succeed())
	}).Should(Succeed())
}
