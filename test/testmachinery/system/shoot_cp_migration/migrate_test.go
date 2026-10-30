// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the migration of a shoot by using the "framework/shootmigrationtest.go"
		- When --live-migration is set, also ensures there is no kube-apiserver downtime during the migration.
		- Performs sanity checks on the migrated shoot to verify that migration is working as expected.
 **/

package cp_migration_test

import (
	"context"
	"flag"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	. "github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/applications"
	shootupdate "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

const (
	ControlPlaneMigrationTimeout = 2 * time.Hour

	zeroDownTimeValidatorJobName      = "live-migration"
	zeroDownTimeValidatorJobNamespace = metav1.NamespaceDefault
)

var (
	targetSeedName             *string
	shootName                  *string
	shootNamespace             *string
	mrExcludeList              *string
	resourcesWithGeneratedName *string
	addTestRunTaint            *string
	liveMigration              *bool
	gardenerConfig             *GardenerConfig
)

func init() {
	gardenerConfig = RegisterGardenerFrameworkFlags()
	targetSeedName = flag.String("target-seed-name", "", "name of the seed to which the shoot will be migrated")
	shootName = flag.String("shoot-name", "", "name of the shoot")
	shootNamespace = flag.String("shoot-namespace", "", "namespace of the shoot")
	mrExcludeList = flag.String("mr-exclude-list", "", "comma-separated values of the ManagedResources that will be exclude during the 'CreationTimestamp' check")
	resourcesWithGeneratedName = flag.String("resources-with-generated-name", "", "comma-separated names of resources deployed via managed resources that get their name generated during reconciliation and will be excluded during the 'CreationTimestamp' check")
	addTestRunTaint = flag.String("add-test-run-taint", "", "if this property is set to 'true' the 'test.gardener.cloud/test-run' taint with the value of the 'TM_TESTRUN_ID' environment variable, will be applied to the shoot")
	liveMigration = flag.Bool("live-migration", false, "if true, performs a live control plane migration ensuring zero kube-apiserver downtime")
}

var _ = ginkgo.Describe("Shoot migration testing", func() {
	var (
		t *ShootMigrationTest

		f            = NewGardenerFramework(gardenerConfig)
		guestBookApp = applications.GuestBookTest{}
	)

	CBeforeEach(func(_ context.Context) {
		validateConfig()
	}, 1*time.Minute)
	CJustBeforeEach(func(ctx context.Context) {
		var err error
		t, err = NewShootMigrationTest(ctx, f, &ShootMigrationConfig{
			ShootName:       *shootName,
			ShootNamespace:  *shootNamespace,
			TargetSeedName:  *targetSeedName,
			AddTestRunTaint: *addTestRunTaint,
		})
		if err != nil {
			ginkgo.Fail("Unable to initialize the shoot migration test: " + err.Error())
		}
		if *liveMigration {
			validateShootForLiveMigration(t)
		}
		if err = beforeMigration(ctx, t, &guestBookApp); err != nil {
			ginkgo.Fail("The Shoot CP Migration preparation steps failed with: " + err.Error())
		}
	}, 15*time.Minute)
	CAfterEach(func(ctx context.Context) {
		if err := afterMigration(ctx, t, guestBookApp); err != nil {
			ginkgo.Fail("The Shoot CP Migration health checks failed with: " + err.Error())
		}
	}, 15*time.Minute)

	CIt("Migrate Shoot", func(ctx context.Context) {
		if err := t.MigrateShoot(ctx, *liveMigration); err != nil {
			ginkgo.Fail("Shoot CP Migration failed with: " + err.Error())
		}
	}, ControlPlaneMigrationTimeout)
})

func validateConfig() {
	if !StringSet(*targetSeedName) {
		ginkgo.Fail("You should specify a name for the target Seed")
	}
	if !StringSet(*shootName) {
		ginkgo.Fail("You should specify a name for the Shoot that will be migrated")
	}
	if !StringSet(*shootNamespace) {
		ginkgo.Fail("You should specify a namespace of the Shoot that will be migrated")
	}
}

func validateShootForLiveMigration(t *ShootMigrationTest) {
	if t.Shoot.Status.IsHibernated {
		ginkgo.Fail("Live control plane migration is not supported for hibernated shoots")
	}
	if !v1beta1helper.IsHAControlPlaneConfigured(&t.Shoot) {
		ginkgo.Fail("Live control plane migration requires the shoot control plane to be configured for high availability")
	}
}

func beforeMigration(ctx context.Context, t *ShootMigrationTest, guestBookApp *applications.GuestBookTest) error {
	if err := t.PopulateBeforeMigrationComparisonElements(ctx); err != nil {
		return err
	}

	if t.Shoot.Status.IsHibernated {
		return nil
	}

	ginkgo.By("Create test Secret and Service Account")
	if err := t.CreateSecretAndServiceAccount(ctx); err != nil {
		return err
	}

	ginkgo.By("Verify Guest Book Application")
	initializedApp := initGuestBookTest(t)
	*guestBookApp = *initializedApp
	guestBookApp.DeployGuestBookApp(ctx)
	guestBookApp.Test(ctx)

	if *liveMigration {
		ginkgo.By("Deploy zero-downtime validator job inside the shoot to ensure no kube-apiserver downtime during live migration")
		job, err := highavailability.DeployZeroDownTimeValidatorJob(ctx, t.ShootClient.Client(), zeroDownTimeValidatorJobName, zeroDownTimeValidatorJobNamespace, "")
		if err != nil {
			return err
		}
		ginkgo.By("Wait until the zero-downtime validator job is ready")
		shootupdate.WaitForJobToBeReady(ctx, t.ShootClient.Client(), job)
	}

	return nil
}

func afterMigration(ctx context.Context, t *ShootMigrationTest, guestBookApp applications.GuestBookTest) error {
	if ginkgo.CurrentSpecReport().Failed() {
		t.GardenerFramework.DumpState(ctx)
		return cleanUp(ctx, t, guestBookApp)
	}

	ginkgo.By("Verifying migration")
	if err := t.VerifyMigration(ctx); err != nil {
		return err
	}

	if *liveMigration {
		ginkgo.By("Ensure there was no kube-apiserver downtime during the live migration")
		job := highavailability.EmptyZeroDownTimeValidatorJob(zeroDownTimeValidatorJobName, zeroDownTimeValidatorJobNamespace)
		if err := t.ShootClient.Client().Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
			return err
		}
		Expect(job.Status.Failed).To(BeZero(), "the kube-apiserver experienced downtime during the live control plane migration")
	}

	if t.Shoot.Status.IsHibernated {
		return nil
	}

	ginkgo.By("Check if the test Secret and Service Account are migrated")
	if err := t.CheckSecretAndServiceAccount(ctx); err != nil {
		return err
	}

	ginkgo.By("Test the Guest Book Application")
	guestBookApp.Test(ctx)

	ginkgo.By("Check timestamps of all resources")
	if err := t.CheckObjectsTimestamp(ctx, strings.Split(*mrExcludeList, ","), strings.Split(*resourcesWithGeneratedName, ",")); err != nil {
		return err
	}

	return cleanUp(ctx, t, guestBookApp)
}

func cleanUp(ctx context.Context, t *ShootMigrationTest, guestBookApp applications.GuestBookTest) error {
	ginkgo.By("Cleanup the test Secret and Service Account")
	if err := t.CleanUpSecretAndServiceAccount(ctx); err != nil {
		return err
	}
	ginkgo.By("Cleanup the Guest Book Application")
	guestBookApp.Cleanup(ctx)

	if *liveMigration {
		ginkgo.By("Cleanup the zero-downtime validator job")
		job := highavailability.EmptyZeroDownTimeValidatorJob(zeroDownTimeValidatorJobName, zeroDownTimeValidatorJobNamespace)
		if err := t.ShootClient.Client().Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

func initGuestBookTest(t *ShootMigrationTest) *applications.GuestBookTest {
	sFramework := ShootFramework{
		GardenerFramework: t.GardenerFramework,
		TestDescription:   NewTestDescription("Guestbook App for CP Migration test"),
		Shoot:             &t.Shoot,
		Seed:              t.SourceSeed,
		ShootClient:       t.ShootClient,
		SeedClient:        t.SourceSeedClient,
	}
	return applications.NewGuestBookTest(&sFramework)
}
