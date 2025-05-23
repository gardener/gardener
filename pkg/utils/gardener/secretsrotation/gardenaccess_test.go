// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
)

var _ = Describe("RenewGardenAccess", func() {
	const (
		renewGardenAccessSecrets    = "renew-garden-access-secrets"
		renewKubeconfig             = "renew-kubeconfig"
		renewWorkloadIdentityTokens = "renew-workload-identity-tokens"

		secretType          = "secretType"
		gardenAccess        = "garden access"
		gardenletKubeconfig = "gardenlet kubeconfig"
		workloadIdentity    = "workload identity"
	)

	var (
		ctx    context.Context
		logger logr.Logger

		gardenClient client.Client

		seeds []gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctx = context.TODO()
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		seeds = []gardencorev1beta1.Seed{
			{ObjectMeta: metav1.ObjectMeta{Name: "seed1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "seed2"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "seed3"}},
		}
	})

	createSeeds := func() error {
		for _, seed := range seeds {
			if err := gardenClient.Create(ctx, &seed); err != nil {
				return err
			}
		}
		return nil
	}

	Context("#CheckIfGardenSecretsRenewalCompletedInAllSeeds", func() {
		It("should succeed if no seed is annotated anymore - `renew-garden-access-secrets`", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(Succeed())
		})

		It("should succeed if no seed is annotated anymore - `renew-kubeconfig`", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, gardenClient, renewKubeconfig, gardenletKubeconfig)).To(Succeed())
		})

		It("should succeed if no seed is annotated anymore - `renew-workload-identity-tokens`", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, gardenClient, renewWorkloadIdentityTokens, workloadIdentity)).To(Succeed())
		})

		It("should succeed if some seeds have a different `gardener.cloud/operation` annotation", func() {
			seeds[0].SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			Expect(createSeeds()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(Succeed())
		})

		It("should fail if some seeds are still annotated with `renew-garden-access-secrets`", func() {
			seeds[1].SetAnnotations(map[string]string{"gardener.cloud/operation": renewGardenAccessSecrets})
			Expect(createSeeds()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(MatchError(ContainSubstring("renewing \"garden access\" secrets for seed \"seed2\" is not yet completed")))
		})
	})

	Context("#RenewGardenSecretsInAllSeeds", func() {
		It("should succeed and annotate all seeds - `renew-garden-access-secrets`", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(RenewGardenSecretsInAllSeeds(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(Succeed())

			seedList := gardencorev1beta1.SeedList{}
			Expect(gardenClient.List(ctx, &seedList)).To(Succeed())
			for _, seed := range seedList.Items {
				Expect(seed.Annotations["gardener.cloud/operation"]).To(Equal(renewGardenAccessSecrets))
			}
		})

		It("should succeed and annotate all seeds - `renew-kubeconfig`", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(RenewGardenSecretsInAllSeeds(ctx, logger.WithValues(secretType, gardenletKubeconfig), gardenClient, renewKubeconfig)).To(Succeed())

			seedList := gardencorev1beta1.SeedList{}
			Expect(gardenClient.List(ctx, &seedList)).To(Succeed())
			for _, seed := range seedList.Items {
				Expect(seed.Annotations["gardener.cloud/operation"]).To(Equal(renewKubeconfig))
			}
		})

		It("should succeed and annotate all seeds - `renew-workload-identity-tokens`", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(RenewGardenSecretsInAllSeeds(ctx, logger.WithValues(secretType, gardenletKubeconfig), gardenClient, renewWorkloadIdentityTokens)).To(Succeed())

			seedList := gardencorev1beta1.SeedList{}
			Expect(gardenClient.List(ctx, &seedList)).To(Succeed())
			for _, seed := range seedList.Items {
				Expect(seed.Annotations["gardener.cloud/operation"]).To(Equal(renewWorkloadIdentityTokens))
			}
		})

		It("should succeed if some seeds are already annotated with `renew-garden-access-secrets`", func() {
			seeds[0].SetAnnotations(map[string]string{"gardener.cloud/operation": renewGardenAccessSecrets})
			Expect(createSeeds()).To(Succeed())

			Expect(RenewGardenSecretsInAllSeeds(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(Succeed())

			seedList := gardencorev1beta1.SeedList{}
			Expect(gardenClient.List(ctx, &seedList)).To(Succeed())
			for _, seed := range seedList.Items {
				Expect(seed.Annotations["gardener.cloud/operation"]).To(Equal(renewGardenAccessSecrets))
			}
		})

		It("should fail if some seeds have a different `gardener.cloud/operation` annotation", func() {
			seeds[0].SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			Expect(createSeeds()).To(Succeed())

			Expect(RenewGardenSecretsInAllSeeds(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(MatchError(ContainSubstring("error annotating seed seed1: already annotated with \"gardener.cloud/operation: reconcile\"")))
		})
	})
})
