// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	shootmigration "github.com/gardener/gardener/test/utils/shoots/migration"
)

// Recreate local2's Kind node to lose Seed ETCD data while retaining its host-mounted backup bucket.
// The force-restore overlay enables periodic ShootState backups before the disaster.
var _ = Describe("Shoot Tests", Label("Shoot", "control-plane-force-restore"), func() {
	test := func(s *ShootContext) {
		s.Shoot.Spec.SeedName = new(getSeedName(true))

		BeforeAll(func() {
			s.Init()
		})

		ItShouldCreateShoot(s)
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldGetResponsibleSeed(s)
		seed.ItShouldInitializeSeedClient(s.SeedContext)

		var secretsBeforeRestore map[string]corev1.Secret

		It("Populate comparison elements before restore", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				var err error
				secretsBeforeRestore, err = shootmigration.GetPersistedSecrets(ctx, s.SeedClientSet.Client(), s.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		// Block ordinary reconciliation against the empty Seed before recovery can regenerate secrets.
		seed.ItShouldAnnotateSeed(s.SeedContext, map[string]string{
			v1beta1constants.AnnotationEmergencyStopShootReconciliations: "true",
		})

		It("Simulate a Seed disaster by destroying and recreating the local2 node", func(ctx SpecContext) {
			By("Destroy the local2 node (wipes the seed etcd)")
			runMake(ctx, nil, "kind2-down")

			By("Recreate the local2 node (re-mounts the surviving backup bucket)")
			runMake(ctx, nil, "kind2-up")

			// Defer Seed readiness until gardenlet is re-bootstrapped below.
			By("Redeploy the local2 gardenlet (kubeconfig update + Gardenlet CRD)")
			runMake(ctx, []string{"GARDENLET_PROFILE_SUFFIX=force-restore"}, "seed2-up")

			// Use a fresh kubeconfig Secret name to avoid the operator's cleanup of the initial bootstrap Secret.
			By("Trigger gardenlet re-bootstrap by annotating Gardenlet/local2 with force-redeploy")
			kubeconfigPath := filepath.Join("..", "..", "..", "dev-setup", "gardenlet", "components", "kubeconfigs", "seed-local2", "kubeconfig-gardener-operator")
			kubeconfigData, err := os.ReadFile(kubeconfigPath) // #nosec G304 -- Fixed repository-relative path controlled by the test.
			Expect(err).NotTo(HaveOccurred())

			const forceRedeployKubeconfigSecret = "seed-local2-force-redeploy" // #nosec G101 -- Secret name, not a credential.
			Eventually(ctx, func() error {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: forceRedeployKubeconfigSecret, Namespace: "garden"},
				}
				if err := s.GardenClient.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
					return err
				}
				secret.Data = map[string][]byte{"kubeconfig": kubeconfigData}
				if err := s.GardenClient.Create(ctx, secret); err != nil {
					return err
				}

				gardenlet := &seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{Name: s.SeedContext.Seed.Name, Namespace: "garden"}}
				if err := s.GardenClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet); err != nil {
					return err
				}
				patch := client.MergeFrom(gardenlet.DeepCopy())
				gardenlet.Spec.KubeconfigSecretRef = &corev1.LocalObjectReference{Name: forceRedeployKubeconfigSecret}
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.OperationForceRedeploy)
				return s.GardenClient.Patch(ctx, gardenlet, patch)
			}).Should(Succeed())
		}, SpecTimeout(20*time.Minute))

		seed.ItShouldWaitForSeedToBeReady(s.SeedContext)
		// Kind recreation changes the Seed API endpoint and CA.
		seed.ItShouldInitializeSeedClient(s.SeedContext)

		ItShouldAnnotateShoot(s, map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationForceRestore,
		})

		It("Remove the emergency-stop annotation from Seed local2", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				if err := s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.SeedContext.Seed), s.SeedContext.Seed); err != nil {
					return err
				}
				patch := client.MergeFrom(s.SeedContext.Seed.DeepCopy())
				delete(s.SeedContext.Seed.Annotations, v1beta1constants.AnnotationEmergencyStopShootReconciliations)
				return s.GardenClient.Patch(ctx, s.SeedContext.Seed, patch)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForShootToBeReconciledAndHealthy(s)

		It("Verify that the force-restore annotation was removed from the Shoot", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())
				g.Expect(s.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			}).WithPolling(10 * time.Second).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Verify that all secrets have been restored without regeneration", func(ctx SpecContext) {
			var secretsAfterRestore map[string]corev1.Secret
			Eventually(ctx, func() error {
				var err error
				secretsAfterRestore, err = shootmigration.GetPersistedSecrets(ctx, s.SeedClientSet.Client(), s.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
			Expect(shootmigration.ComparePersistedSecrets(secretsBeforeRestore, secretsAfterRestore)).To(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldDeleteShoot(s)
		ItShouldWaitForShootToBeDeleted(s)
	}

	Context("Shoot with workers", Ordered, PriorityLonger, func() {
		test(NewTestContext().ForShoot(DefaultShoot("e2e-frestore")))
	})
})

// runMake executes a repository-root target with additional environment variables and streams its output.
func runMake(ctx SpecContext, extraEnv []string, target string) {
	GinkgoHelper()

	cmd := exec.CommandContext(ctx, "make", target) // #nosec G204 -- Used for e2e tests only.
	cmd.Dir = filepath.Join("..", "..", "..")
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = gexec.NewPrefixedWriter("[make "+target+" out] ", GinkgoWriter)
	cmd.Stderr = gexec.NewPrefixedWriter("[make "+target+" err] ", GinkgoWriter)
	Expect(cmd.Run()).To(Succeed())
}
