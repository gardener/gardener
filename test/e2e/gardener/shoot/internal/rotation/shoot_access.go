// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"flag"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/access"
)

type clients struct {
	adminKubeconfig, clientCert, serviceAccountDynamic, serviceAccountStatic kubernetes.Interface
}

// ShootAccessVerifier uses the various access methods to access the Shoot.
type ShootAccessVerifier struct {
	*ShootContext

	clientsBefore, clientsPrepared, clientsAfter clients
}

// Before is called before the rotation is started.
func (v *ShootAccessVerifier) Before(_ context.Context) {
	It("Use admin kubeconfig with old CA to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClientSet, v.Shoot)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsBefore.adminKubeconfig = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new client certificate and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsBefore.adminKubeconfig, "e2e-rotate-csr-before")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsBefore.clientCert = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new dynamic token for a ServiceAccount and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsBefore.adminKubeconfig, "e2e-rotate-sa-dynamic-before")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsBefore.serviceAccountDynamic = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new static token for a ServiceAccount and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, v.clientsBefore.adminKubeconfig, "e2e-rotate-sa-static-before")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsBefore.serviceAccountStatic = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ShootAccessVerifier) ExpectPreparingStatus(_ Gomega) {}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *ShootAccessVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *ShootAccessVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ShootAccessVerifier) AfterPrepared(_ context.Context) {
	It("Use admin kubeconfig with old CA to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use client certificate from before rotation to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use dynamic ServiceAccount token from before rotation to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use static ServiceAccount token from before rotation to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.serviceAccountStatic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use admin kubeconfig with CA bundle to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClientSet, v.Shoot)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsPrepared.adminKubeconfig = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new client certificate and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsPrepared.adminKubeconfig, "e2e-rotate-csr-prepared")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsPrepared.clientCert = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new dynamic token for a ServiceAccount and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsPrepared.adminKubeconfig, "e2e-rotate-sa-dynamic-prepared")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsPrepared.serviceAccountDynamic = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new static token for a ServiceAccount and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, v.clientsPrepared.adminKubeconfig, "e2e-rotate-sa-static-prepared")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsPrepared.serviceAccountStatic = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ShootAccessVerifier) ExpectCompletingStatus(_ Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ShootAccessVerifier) AfterCompleted(_ context.Context) {
	It("Use admin kubeconfig with old CA to access shoot", func(ctx SpecContext) {
		Consistently(func(g Gomega) {
			g.Expect(v.clientsBefore.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use client certificate from before rotation to access shoot", func(ctx SpecContext) {
		Consistently(func(g Gomega) {
			g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use dynamic ServiceAccount token from before rotation to access shoot", func(ctx SpecContext) {
		Consistently(func(g Gomega) {
			g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use static ServiceAccount token from before rotation to access shoot", func(ctx SpecContext) {
		Consistently(func(g Gomega) {
			g.Expect(v.clientsBefore.serviceAccountStatic.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use admin kubeconfig with CA bundle to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsPrepared.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	})

	It("Use client certificate from after preparation to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsPrepared.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use dynamic ServiceAccount token from after preparation to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsPrepared.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use static ServiceAccount token from after preparation to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsPrepared.serviceAccountStatic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use admin kubeconfig with new CA to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClientSet, v.Shoot)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsAfter.adminKubeconfig = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new client certificate and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsAfter.adminKubeconfig, "e2e-rotate-csr-after")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsAfter.clientCert = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new dynamic token for a ServiceAccount and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsAfter.adminKubeconfig, "e2e-rotate-sa-dynamic-after")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsAfter.serviceAccountDynamic = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new static token for a ServiceAccount and using it to access shoot", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, v.clientsAfter.adminKubeconfig, "e2e-rotate-sa-static-after")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsAfter.serviceAccountStatic = shootClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// Cleanup is passed to ginkgo.DeferCleanup.
func (v *ShootAccessVerifier) Cleanup() {
	// TODO(Wieneo): drop this lookup and use the new flags from the e2e package once the test/e2e/gardener package no longer uses
	// the test framework (when finishing https://github.com/gardener/gardener/issues/11379)
	if flag.Lookup("existing-shoot-name").Value.String() == "" {
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

	It("Clean up objects in shoot from client certificate access", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(access.CleanupObjectsFromCSRAccess(ctx, shootClient)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Clean up objects in shoot from dynamic ServiceAccount token access", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(access.CleanupObjectsFromDynamicServiceAccountTokenAccess(ctx, shootClient)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Clean up objects in shoot from static ServiceAccount token access", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(access.CleanupObjectsFromStaticServiceAccountTokenAccess(ctx, shootClient)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
