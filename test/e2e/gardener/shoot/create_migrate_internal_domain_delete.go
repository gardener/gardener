// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/rotation"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	// This test is Serial because it changes the internal domain of the seed through its Gardenlet resource. This
	// restarts gardenlet, gives the shoots of concurrent specs another internal domain, and the seed admission refuses
	// to remove an internal domain while a shoot uses it.
	Describe("Create Shoot, Migrate, Disable, and Enable its Internal Domain, and Delete Shoot", Label("internal-domain"), Ordered, Serial, PriorityLong, func() {
		// The local setup provides the credentials of this internal domain in addition to the internal domain of the seed.
		const (
			migrationDomain       = "internal-migration.local.gardener.cloud"
			migrationDomainSecret = "internal-domain-internal-migration-local-gardener-cloud"
		)

		var (
			tc          = NewShootContext(DefaultWorkerlessShoot("e2e-int-dom"))
			projectName = strings.TrimPrefix(tc.Shoot.Namespace, "garden-")

			gardenlet               = &seedmanagementv1alpha1.Gardenlet{}
			originalConfig          runtime.RawExtension
			originalInternalDomain  *gardencorev1beta1.SeedDNSProviderConfig
			migrationInternalDomain *gardencorev1beta1.SeedDNSProviderConfig

			migrateVerifier = &rotation.InternalDomainVerifier{ShootContext: tc, NewDomain: internalClusterDomain(tc.Shoot.Name, projectName, migrationDomain)}
			disableVerifier = &rotation.InternalDomainVerifier{ShootContext: tc}
			enableVerifier  = &rotation.InternalDomainVerifier{ShootContext: tc}
		)

		makeOriginalInternalDomainFirst := func(ctx context.Context) {
			patchGardenletSeedDNS(ctx, tc, gardenlet, map[string]any{
				"internal":        originalInternalDomain,
				"internalDomains": []gardencorev1beta1.SeedDNSProviderConfig{*originalInternalDomain, *migrationInternalDomain},
			})
		}

		restoreGardenletConfig := func(ctx context.Context) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
				gardenlet.Spec.Config = *originalConfig.DeepCopy()
				g.Expect(tc.GardenClient.Update(ctx, gardenlet)).To(Succeed())
			}).Should(Succeed())
		}

		haveOriginalInternalDomainFirst := func() gomegatypes.GomegaMatcher {
			return HaveField("InternalDomains", HaveExactElements(
				HaveField("Domain", Not(Equal(migrationDomain))),
				HaveField("Domain", migrationDomain),
			))
		}

		haveOriginalInternalDomainOnly := func() gomegatypes.GomegaMatcher {
			return And(
				HaveField("InternalDomainEnabled", Or(BeNil(), HaveValue(BeTrue()))),
				HaveField("InternalDomains", HaveExactElements(HaveField("Domain", Not(Equal(migrationDomain))))),
			)
		}

		BeforeAll(func() {
			tc.Init()

			DeferCleanup(func(ctx SpecContext) {
				if originalConfig.Raw == nil {
					return
				}
				Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
				if unchanged, err := MatchJSON(originalConfig.Raw).Match(gardenlet.Spec.Config.Raw); err == nil && unchanged {
					return
				}

				makeOriginalInternalDomainFirst(ctx)
				waitForSeedDNS(ctx, tc, haveOriginalInternalDomainFirst())
				restoreGardenletConfig(ctx)
				waitForSeedDNS(ctx, tc, haveOriginalInternalDomainOnly())
			}, NodeTimeout(15*time.Minute))

			DeferCleanup(func(ctx SpecContext) {
				if existingShootName != "" {
					return
				}
				Eventually(ctx, func(g Gomega) {
					g.Expect(gardenerutils.ConfirmDeletion(ctx, tc.GardenClient, tc.Shoot)).To(Or(Succeed(), BeNotFoundError()))
					g.Expect(tc.GardenClient.Delete(ctx, tc.Shoot)).To(Or(Succeed(), BeNotFoundError()))
				}).Should(Succeed())
				Eventually(ctx, tc.GardenKomega.Get(tc.Shoot)).WithPolling(30 * time.Second).Should(BeNotFoundError())
			}, NodeTimeout(20*time.Minute))
		})

		ItShouldCreateShoot(tc)
		ItShouldWaitForShootToBeReconciledAndHealthy(tc)
		ItShouldGetResponsibleSeed(tc)
		seed.ItShouldInitializeSeedClient(tc.SeedContext)

		It("Remember the gardenlet configuration", func(ctx SpecContext) {
			Expect(tc.Seed.Spec.DNS.Internal).NotTo(BeNil(), "the seed must have an internal domain")
			originalInternalDomain = tc.Seed.Spec.DNS.Internal.DeepCopy()
			enableVerifier.NewDomain = internalClusterDomain(tc.Shoot.Name, projectName, originalInternalDomain.Domain)

			migrationInternalDomain = originalInternalDomain.DeepCopy()
			migrationInternalDomain.Domain = migrationDomain
			migrationInternalDomain.CredentialsRef.Name = migrationDomainSecret

			gardenlet.Name, gardenlet.Namespace = tc.Seed.Name, v1beta1constants.GardenNamespace
			Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
			originalConfig = *gardenlet.Spec.Config.DeepCopy()
		}, SpecTimeout(time.Minute))

		Context("Migrate to a new internal domain", func() {
			It("Give new shoots a new internal domain", func(ctx SpecContext) {
				patchGardenletSeedDNS(ctx, tc, gardenlet, map[string]any{
					"internal":        migrationInternalDomain,
					"internalDomains": []gardencorev1beta1.SeedDNSProviderConfig{*migrationInternalDomain, *originalInternalDomain},
				})
			}, SpecTimeout(time.Minute))

			itShouldWaitForSeedDNS(tc, HaveField("InternalDomains", HaveExactElements(
				HaveField("Domain", migrationDomain),
				HaveField("Domain", Not(Equal(migrationDomain))),
			)))
			testInternalDomainMigration(tc, migrateVerifier)
		})

		Context("Disable the internal domain", func() {
			It("Disable the internal domain on the seed", func(ctx SpecContext) {
				patchGardenletSeedDNS(ctx, tc, gardenlet, map[string]any{"internalDomainEnabled": false})
			}, SpecTimeout(time.Minute))

			itShouldWaitForSeedDNS(tc, HaveField("InternalDomainEnabled", HaveValue(BeFalse())))
			testInternalDomainMigration(tc, disableVerifier)
		})

		Context("Enable the internal domain again", func() {
			// While a shoot is scheduled onto the seed, the admission refuses to remove the first internal domain in the
			// update that replaces it. Thus the original domain becomes the first one before the migration domain is removed.
			It("Make the original internal domain the first one again", func(ctx SpecContext) {
				makeOriginalInternalDomainFirst(ctx)
			}, SpecTimeout(time.Minute))

			itShouldWaitForSeedDNS(tc, haveOriginalInternalDomainFirst())

			It("Restore the gardenlet configuration", func(ctx SpecContext) {
				restoreGardenletConfig(ctx)
			}, SpecTimeout(time.Minute))

			itShouldWaitForSeedDNS(tc, haveOriginalInternalDomainOnly())
			testInternalDomainMigration(tc, enableVerifier)
		})

		ItShouldDeleteShoot(tc)
		ItShouldWaitForShootToBeDeleted(tc)
	})
})

// testInternalDomainMigration confirms the migration of the internal domain and runs it with a certificate authorities
// rotation.
func testInternalDomainMigration(tc *ShootContext, verifier *rotation.InternalDomainVerifier) {
	ItShouldAnnotateShoot(tc, map[string]string{v1beta1constants.AnnotationConfirmationMigrateInternalDomain: "true"})
	testCredentialRotation(tc, rotationutils.Verifiers{verifier}, nil, v1beta1constants.OperationRotateCAStart, v1beta1constants.OperationRotateCAComplete, false)
}

// patchGardenletSeedDNS merges the given fields into the DNS configuration of the seed in the gardenlet configuration.
// gardenlet writes this configuration into the Seed.
func patchGardenletSeedDNS(ctx context.Context, tc *ShootContext, gardenlet *seedmanagementv1alpha1.Gardenlet, dns map[string]any) {
	patch, err := json.Marshal(map[string]any{"spec": map[string]any{"config": map[string]any{"seedConfig": map[string]any{"spec": map[string]any{"dns": dns}}}}})
	Expect(err).NotTo(HaveOccurred())

	Eventually(ctx, func() error {
		return tc.GardenClient.Patch(ctx, gardenlet, client.RawPatch(types.MergePatchType, patch))
	}).Should(Succeed())
}

// itShouldWaitForSeedDNS waits until gardenlet wrote the changed DNS configuration into the Seed, and the Seed is ready.
func itShouldWaitForSeedDNS(tc *ShootContext, matcher gomegatypes.GomegaMatcher) {
	GinkgoHelper()

	It("Wait for the DNS configuration of the seed", func(ctx SpecContext) {
		waitForSeedDNS(ctx, tc, matcher)
	}, SpecTimeout(5*time.Minute))

	seed.ItShouldWaitForSeedToBeReady(tc.SeedContext)
}

func waitForSeedDNS(ctx context.Context, tc *ShootContext, matcher gomegatypes.GomegaMatcher) {
	Eventually(ctx, func(g Gomega) {
		g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(tc.Seed), tc.Seed)).To(Succeed())
		g.Expect(tc.Seed.Spec.DNS).To(matcher)
	}).WithPolling(10 * time.Second).Should(Succeed())
}

func internalClusterDomain(shootName, projectName, internalDomain string) string {
	return *gardenerutils.ConstructInternalClusterDomain(shootName, projectName, &gardenerutils.Domain{Domain: internalDomain})
}
