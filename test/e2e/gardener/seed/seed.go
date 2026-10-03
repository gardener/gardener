// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// ItShouldInitializeSeedClient initializes the context's seed clients from the garden/seed-<name> kubeconfig secret.
// Requires ItShouldGetResponsibleSeed to be called first.
func ItShouldInitializeSeedClient(tc *SeedContext) {
	GinkgoHelper()

	It("Initialize Seed client", func(ctx SpecContext) {
		Expect(tc.Seed).NotTo(BeNil(), "ItShouldGetResponsibleSeed should be called first")

		seedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "seed-" + tc.Seed.Name,
				Namespace: "garden",
			},
		}
		Eventually(ctx, tc.GardenKomega.Object(seedSecret)).Should(
			HaveField("Data", HaveKey(kubernetes.KubeConfig)),
			"secret %v should contain the seed kubeconfig",
		)

		clientSet, err := kubernetes.NewClientFromSecretObject(seedSecret,
			kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
			kubernetes.WithDisabledCachedClient(),
		)
		Expect(err).NotTo(HaveOccurred())
		tc.SetSeedClientSet(clientSet)
	}, SpecTimeout(time.Minute))
}

// ItShouldAnnotateSeed sets the given annotation within the seed metadata to the specified value and patches the seed object
func ItShouldAnnotateSeed(tc *SeedContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate Seed", func(ctx SpecContext) {
		patch := client.MergeFrom(tc.Seed.DeepCopy())

		for key, value := range annotations {
			tc.Log.Info("Setting annotation", "annotation", key, "value", value)
			metav1.SetMetaDataAnnotation(&tc.Seed.ObjectMeta, key, value)
		}

		Eventually(ctx, func() error {
			return tc.GardenClient.Patch(ctx, tc.Seed, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForSeedToBeReady waits for the seed object to be ready
func ItShouldWaitForSeedToBeReady(tc *SeedContext) {
	GinkgoHelper()

	It("Should wait for seed to be ready", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(tc.Seed), tc.Seed)).To(Succeed())
			g.Expect(health.CheckSeed(tc.Seed, tc.Seed.Status.Gardener)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(10*time.Minute))
}

// ItShouldWaitForSeedToBeDeleted waits for the seed object to be gone
func ItShouldWaitForSeedToBeDeleted(tc *SeedContext) {
	GinkgoHelper()

	It("Wait for Seed to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := tc.GardenKomega.Get(tc.Seed)()
			if err == nil {
				tc.Log.Info("Waiting for deletion", "lastOperation", tc.Seed.Status.LastOperation)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		tc.Log.Info("Seed has been deleted")
	}, SpecTimeout(10*time.Minute))
}

// ItShouldCreatePrometheusRuleForSeed creates a PrometheusRule and makes sure it is created.
func ItShouldCreatePrometheusRuleForSeed(tc *SeedContext, rule *monitoringv1.PrometheusRule) {
	GinkgoHelper()

	It("Create PrometheusRule "+rule.Namespace+"/"+rule.Name, func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Create(ctx, rule)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeletePrometheusRuleForSeed deletes a PrometheusRule and makes sure it is deleted.
func ItShouldDeletePrometheusRuleForSeed(tc *SeedContext, rule *monitoringv1.PrometheusRule) {
	GinkgoHelper()

	It("Delete PrometheusRule "+rule.Namespace+"/"+rule.Name, func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Delete(ctx, rule)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
