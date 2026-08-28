// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
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

	Context("#RenewKubeconfigInAllShootGardenlets", func() {
		var gardenlets []seedmanagementv1alpha1.Gardenlet

		BeforeEach(func() {
			gardenlets = []seedmanagementv1alpha1.Gardenlet{
				{ObjectMeta: metav1.ObjectMeta{Name: "self-hosted-shoot-g1", Namespace: "garden"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "self-hosted-shoot-g2", Namespace: "garden"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "self-hosted-shoot-g3", Namespace: "garden"}},
			}
		})

		createGardenlets := func() error {
			for _, gardenlet := range gardenlets {
				if err := gardenClient.Create(ctx, &gardenlet); err != nil {
					return err
				}
			}
			return nil
		}

		It("should succeed and annotate all self-hosted-shoot gardenlets", func() {
			Expect(createGardenlets()).To(Succeed())

			Expect(RenewKubeconfigInAllShootGardenlets(ctx, logger.WithValues(secretType, gardenletKubeconfig), gardenClient)).To(Succeed())

			gardenletList := seedmanagementv1alpha1.GardenletList{}
			Expect(gardenClient.List(ctx, &gardenletList)).To(Succeed())
			for _, gardenlet := range gardenletList.Items {
				Expect(gardenlet.Annotations["gardener.cloud/operation"]).To(Equal(renewKubeconfig))
			}
		})

		It("should succeed if some gardenlets are already annotated with `renew-kubeconfig`", func() {
			gardenlets[0].SetAnnotations(map[string]string{"gardener.cloud/operation": renewKubeconfig})
			Expect(createGardenlets()).To(Succeed())

			Expect(RenewKubeconfigInAllShootGardenlets(ctx, logger.WithValues(secretType, gardenletKubeconfig), gardenClient)).To(Succeed())

			gardenletList := seedmanagementv1alpha1.GardenletList{}
			Expect(gardenClient.List(ctx, &gardenletList)).To(Succeed())
			for _, gardenlet := range gardenletList.Items {
				Expect(gardenlet.Annotations["gardener.cloud/operation"]).To(Equal(renewKubeconfig))
			}
		})

		It("should fail if some gardenlets have a different `gardener.cloud/operation` annotation", func() {
			gardenlets[0].SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			Expect(createGardenlets()).To(Succeed())

			Expect(RenewKubeconfigInAllShootGardenlets(ctx, logger.WithValues(secretType, gardenletKubeconfig), gardenClient)).To(MatchError(ContainSubstring("error annotating gardenlet garden/self-hosted-shoot-g1: already annotated with \"gardener.cloud/operation: reconcile\"")))
		})

		It("should skip gardenlets not related to self-hosted shoots", func() {
			nonSelfHosted := seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{Name: "managed-seed-gardenlet", Namespace: "garden"}}
			Expect(gardenClient.Create(ctx, &nonSelfHosted)).To(Succeed())
			Expect(createGardenlets()).To(Succeed())

			Expect(RenewKubeconfigInAllShootGardenlets(ctx, logger.WithValues(secretType, gardenletKubeconfig), gardenClient)).To(Succeed())

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(&nonSelfHosted), updated)).To(Succeed())
			Expect(updated.Annotations["gardener.cloud/operation"]).To(BeEmpty())
		})
	})

	Context("#CheckIfKubeconfigRenewalCompletedInAllShootGardenlets", func() {
		var gardenlets []seedmanagementv1alpha1.Gardenlet

		BeforeEach(func() {
			gardenlets = []seedmanagementv1alpha1.Gardenlet{
				{ObjectMeta: metav1.ObjectMeta{Name: "self-hosted-shoot-g1", Namespace: "garden"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "self-hosted-shoot-g2", Namespace: "garden"}},
			}
		})

		createGardenlets := func() error {
			for _, gardenlet := range gardenlets {
				if err := gardenClient.Create(ctx, &gardenlet); err != nil {
					return err
				}
			}
			return nil
		}

		It("should succeed if no gardenlet is annotated anymore", func() {
			Expect(createGardenlets()).To(Succeed())

			Expect(CheckIfKubeconfigRenewalCompletedInAllShootGardenlets(ctx, gardenClient)).To(Succeed())
		})

		It("should succeed if some gardenlets have a different `gardener.cloud/operation` annotation", func() {
			gardenlets[0].SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			Expect(createGardenlets()).To(Succeed())

			Expect(CheckIfKubeconfigRenewalCompletedInAllShootGardenlets(ctx, gardenClient)).To(Succeed())
		})

		It("should fail if some gardenlets are still annotated with `renew-kubeconfig`", func() {
			gardenlets[1].SetAnnotations(map[string]string{"gardener.cloud/operation": renewKubeconfig})
			Expect(createGardenlets()).To(Succeed())

			Expect(CheckIfKubeconfigRenewalCompletedInAllShootGardenlets(ctx, gardenClient)).To(MatchError(ContainSubstring("renewing kubeconfig for Gardenlet garden/self-hosted-shoot-g2 is not yet completed")))
		})

		It("should succeed if only non-self-hosted-shoot gardenlets are still annotated with `renew-kubeconfig`", func() {
			nonSelfHosted := seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{
				Name:        "managed-seed-gardenlet",
				Namespace:   "garden",
				Annotations: map[string]string{"gardener.cloud/operation": renewKubeconfig},
			}}
			Expect(gardenClient.Create(ctx, &nonSelfHosted)).To(Succeed())
			Expect(createGardenlets()).To(Succeed())

			Expect(CheckIfKubeconfigRenewalCompletedInAllShootGardenlets(ctx, gardenClient)).To(Succeed())
		})
	})

	Context("#RenewGardenSecretsInAllSelfHostedShoots", func() {
		var shoots []gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoots = []gardencorev1beta1.Shoot{
				{ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden", Labels: map[string]string{"shoot.gardener.cloud/self-hosted": "true"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "shoot2", Namespace: "garden", Labels: map[string]string{"shoot.gardener.cloud/self-hosted": "true"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "shoot3", Namespace: "garden", Labels: map[string]string{"shoot.gardener.cloud/self-hosted": "true"}}},
			}
		})

		createShoots := func() error {
			for _, shoot := range shoots {
				if err := gardenClient.Create(ctx, &shoot); err != nil {
					return err
				}
			}
			return nil
		}

		It("should succeed and annotate all self-hosted shoots", func() {
			Expect(createShoots()).To(Succeed())

			Expect(RenewGardenSecretsInAllSelfHostedShoots(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(Succeed())

			shootList := gardencorev1beta1.ShootList{}
			Expect(gardenClient.List(ctx, &shootList)).To(Succeed())
			for _, shoot := range shootList.Items {
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(Equal(renewGardenAccessSecrets))
			}
		})

		It("should succeed if some shoots are already annotated with the operation", func() {
			shoots[0].SetAnnotations(map[string]string{"gardener.cloud/operation": renewGardenAccessSecrets})
			Expect(createShoots()).To(Succeed())

			Expect(RenewGardenSecretsInAllSelfHostedShoots(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(Succeed())

			shootList := gardencorev1beta1.ShootList{}
			Expect(gardenClient.List(ctx, &shootList)).To(Succeed())
			for _, shoot := range shootList.Items {
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(Equal(renewGardenAccessSecrets))
			}
		})

		It("should succeed and append the operation if some shoots are already annotated with a different operation", func() {
			shoots[0].SetAnnotations(map[string]string{"gardener.cloud/operation": renewWorkloadIdentityTokens})
			Expect(createShoots()).To(Succeed())

			Expect(RenewGardenSecretsInAllSelfHostedShoots(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKey{Name: "shoot1", Namespace: "garden"}, updated)).To(Succeed())
			Expect(updated.Annotations["gardener.cloud/operation"]).To(Equal(renewWorkloadIdentityTokens + ";" + renewGardenAccessSecrets))

			shootList := gardencorev1beta1.ShootList{}
			Expect(gardenClient.List(ctx, &shootList)).To(Succeed())
			for _, shoot := range shootList.Items {
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(ContainSubstring(renewGardenAccessSecrets))
			}
		})

		It("should skip non-self-hosted shoots", func() {
			nonSelfHosted := gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "regular-shoot", Namespace: "garden"}}
			Expect(gardenClient.Create(ctx, &nonSelfHosted)).To(Succeed())
			Expect(createShoots()).To(Succeed())

			Expect(RenewGardenSecretsInAllSelfHostedShoots(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(&nonSelfHosted), updated)).To(Succeed())
			Expect(updated.Annotations["gardener.cloud/operation"]).To(BeEmpty())
		})

		It("should skip self-hosted shoots that are also registered as seeds", func() {
			seed := gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "shoot1"}}
			Expect(gardenClient.Create(ctx, &seed)).To(Succeed())
			Expect(createShoots()).To(Succeed())

			Expect(RenewGardenSecretsInAllSelfHostedShoots(ctx, logger.WithValues(secretType, gardenAccess), gardenClient, renewGardenAccessSecrets)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKey{Name: "shoot1", Namespace: "garden"}, updated)).To(Succeed())
			Expect(updated.Annotations["gardener.cloud/operation"]).To(BeEmpty())
		})
	})

	Context("#CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots", func() {
		var shoots []gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoots = []gardencorev1beta1.Shoot{
				{ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden", Labels: map[string]string{"shoot.gardener.cloud/self-hosted": "true"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "shoot2", Namespace: "garden", Labels: map[string]string{"shoot.gardener.cloud/self-hosted": "true"}}},
			}
		})

		createShoots := func() error {
			for _, shoot := range shoots {
				if err := gardenClient.Create(ctx, &shoot); err != nil {
					return err
				}
			}
			return nil
		}

		It("should succeed if no self-hosted shoot is annotated anymore", func() {
			Expect(createShoots()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(Succeed())
		})

		It("should succeed if some shoots have a different `gardener.cloud/operation` annotation", func() {
			shoots[0].SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			Expect(createShoots()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(Succeed())
		})

		It("should fail if some self-hosted shoots are still annotated", func() {
			shoots[1].SetAnnotations(map[string]string{"gardener.cloud/operation": renewGardenAccessSecrets})
			Expect(createShoots()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(MatchError(ContainSubstring("renewing \"garden access\" secrets for self-hosted shoot garden/shoot2 is not yet completed")))
		})

		It("should fail if some self-hosted shoots have a combined annotation that still includes the operation", func() {
			shoots[1].SetAnnotations(map[string]string{"gardener.cloud/operation": renewWorkloadIdentityTokens + ";" + renewGardenAccessSecrets})
			Expect(createShoots()).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(MatchError(ContainSubstring("renewing \"garden access\" secrets for self-hosted shoot garden/shoot2 is not yet completed")))
		})

		It("should succeed if only a self-hosted shoot registered as a seed is still annotated", func() {
			shoots[1].SetAnnotations(map[string]string{"gardener.cloud/operation": renewGardenAccessSecrets})
			Expect(createShoots()).To(Succeed())
			seed := gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "shoot2"}}
			Expect(gardenClient.Create(ctx, &seed)).To(Succeed())

			Expect(CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots(ctx, gardenClient, renewGardenAccessSecrets, gardenAccess)).To(Succeed())
		})
	})
})
