// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/utils/access"
)

type clients struct {
	accessSecret, clientCert, serviceAccountDynamic kubernetes.Interface
}

// VirtualGardenAccessVerifier uses the various access methods to access the virtual garden.
type VirtualGardenAccessVerifier struct {
	RuntimeClient client.Client
	Namespace     string

	clientsBefore, clientsPrepared, clientsAfter clients
}

// Before is called before the rotation is started.
func (v *VirtualGardenAccessVerifier) Before(ctx context.Context) {
	var err error
	v.clientsBefore.accessSecret, err = kubernetes.NewClientFromSecret(ctx, v.RuntimeClient, v.Namespace, "gardener", kubernetes.WithDisabledCachedClient())
	Expect(err).NotTo(HaveOccurred())

	By("Request new client certificate and using it to access virtual garden")
	Eventually(func(g Gomega) {
		virtualGardenClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsBefore.accessSecret, "e2e-rotate-csr-before")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.clientCert = virtualGardenClient
	}).Should(Succeed())

	By("Request new dynamic token for a ServiceAccount and using it to access target cluster")
	Eventually(func(g Gomega) {
		virtualGardenClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsBefore.accessSecret, "e2e-rotate-sa-dynamic-before")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.serviceAccountDynamic = virtualGardenClient
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *VirtualGardenAccessVerifier) ExpectPreparingStatus(_ Gomega) {}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *VirtualGardenAccessVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *VirtualGardenAccessVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *VirtualGardenAccessVerifier) AfterPrepared(ctx context.Context) {
	By("Use client certificate from before rotation to access target cluster")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use dynamic ServiceAccount token from before rotation to access target cluster")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	var err error
	v.clientsPrepared.accessSecret, err = kubernetes.NewClientFromSecret(ctx, v.RuntimeClient, v.Namespace, "gardener", kubernetes.WithDisabledCachedClient())
	Expect(err).NotTo(HaveOccurred())

	By("Request new client certificate and using it to access target cluster")
	Eventually(func(g Gomega) {
		virtualGardenClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsPrepared.accessSecret, "e2e-rotate-csr-prepared")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.clientCert = virtualGardenClient
	}).Should(Succeed())

	By("Request new dynamic token for a ServiceAccount and using it to access target cluster")
	Eventually(func(g Gomega) {
		virtualGardenClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsPrepared.accessSecret, "e2e-rotate-sa-dynamic-prepared")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.serviceAccountDynamic = virtualGardenClient
	}).Should(Succeed())
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *VirtualGardenAccessVerifier) ExpectCompletingStatus(_ Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *VirtualGardenAccessVerifier) AfterCompleted(ctx context.Context) {
	By("Use client certificate from before rotation to access target cluster")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Use dynamic ServiceAccount token from before rotation to access target cluster")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Use client certificate from after preparation to access target cluster")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Use dynamic ServiceAccount token from after preparation to access target cluster")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	var err error
	v.clientsAfter.accessSecret, err = kubernetes.NewClientFromSecret(ctx, v.RuntimeClient, v.Namespace, "gardener", kubernetes.WithDisabledCachedClient())
	Expect(err).NotTo(HaveOccurred())

	By("Request new client certificate and using it to access target cluster")
	Eventually(func(g Gomega) {
		virtualGardenClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsAfter.accessSecret, "e2e-rotate-csr-after")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsAfter.clientCert = virtualGardenClient
	}).Should(Succeed())

	By("Request new dynamic token for a ServiceAccount and using it to access target cluster")
	Eventually(func(g Gomega) {
		virtualGardenClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsAfter.accessSecret, "e2e-rotate-sa-dynamic-after")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsAfter.serviceAccountDynamic = virtualGardenClient
	}).Should(Succeed())
}

// Cleanup is passed to ginkgo.DeferCleanup.
func (v *VirtualGardenAccessVerifier) Cleanup(ctx context.Context) {
	virtualGardenClient, err := kubernetes.NewClientFromSecret(ctx, v.RuntimeClient, v.Namespace, "gardener", kubernetes.WithDisabledCachedClient())
	Expect(err).NotTo(HaveOccurred())

	By("Clean up objects in virtual garden from client certificate access")
	Eventually(func(g Gomega) {
		g.Expect(access.CleanupObjectsFromCSRAccess(ctx, virtualGardenClient)).To(Succeed())
	}).Should(Succeed())

	By("Clean up objects in virtual garden from dynamic ServiceAccount token access")
	Eventually(func(g Gomega) {
		g.Expect(access.CleanupObjectsFromDynamicServiceAccountTokenAccess(ctx, virtualGardenClient)).To(Succeed())
	}).Should(Succeed())
}
