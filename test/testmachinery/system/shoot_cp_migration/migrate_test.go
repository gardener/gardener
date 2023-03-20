// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
	Overview
		- Tests the migration of a shoot by using the "framework/shootmigrationtest.go"
		- Performs sanity checks on the migrated shoot to verify that migration is working as expected.
 **/

package cp_migration_test

import (
	"context"
	"flag"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/applications"
)

const (
	ControlPlaneMigrationTimeout = 2 * time.Hour
)

var (
	targetSeedName             *string
	shootName                  *string
	shootNamespace             *string
	mrExcludeList              *string
	resourcesWithGeneratedName *string
	addTestRunTaint            *string
	gardenerConfig             *GardenerConfig
)

func init() {
	gardenerConfig = RegisterGardenerFrameworkFlags()
	targetSeedName = flag.String("target-seed-name", "", "name of the seed to which the shoot will be migrated")
	shootName = flag.String("shoot-name", "", "name of the shoot")
	shootNamespace = flag.String("shoot-namespace", "", "namespace of the shoot")
	mrExcludeList = flag.String("mr-exclude-list", "", "comma-separated values of the ManagedResources that will be exlude during the 'CreationTimestamp' check")
	resourcesWithGeneratedName = flag.String("resources-with-generated-name", "", "comma-separated names of resources deployed via managed resources that get their name generated during reconciliation and will be excluded during the 'CreationTimestamp' check")
	addTestRunTaint = flag.String("add-test-run-taint", "", "if this property is set to 'true' the 'test.gardener.cloud/test-run' taint with the value of the 'TM_TESTRUN_ID' environment variable, will be applied to the shoot")
}

var _ = ginkgo.Describe("Shoot migration testing", func() {
	var (
		t *ShootMigrationTest

		f            = NewGardenerFramework(gardenerConfig)
		guestBookApp = applications.GuestBookTest{}
	)

	CBeforeEach(func(c context.Context) {
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
		if err := t.MigrateShoot(ctx); err != nil {
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

func beforeMigration(ctx context.Context, t *ShootMigrationTest, guestBookApp *applications.GuestBookTest) error {
	if t.Shoot.Status.IsHibernated {
		return nil
	}

	ginkgo.By("Create test Secret and Service Account")
	if err := t.CreateSecretAndServiceAccount(ctx); err != nil {
		return err
	}

	ginkgo.By("Verify Guest Book Application")
	initializedApp, err := initGuestBookTest(ctx, t)
	if err != nil {
		return err
	}
	*guestBookApp = *initializedApp
	guestBookApp.DeployGuestBookApp(ctx)
	guestBookApp.Test(ctx)

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

	return nil
}

func initGuestBookTest(ctx context.Context, t *ShootMigrationTest) (*applications.GuestBookTest, error) {
	sFramework := ShootFramework{
		GardenerFramework: t.GardenerFramework,
		TestDescription:   NewTestDescription("Guestbook App for CP Migration test"),
		Shoot:             &t.Shoot,
		Seed:              t.SourceSeed,
		ShootClient:       t.ShootClient,
		SeedClient:        t.SourceSeedClient,
	}
	if !t.Shoot.Spec.Addons.NginxIngress.Enabled {
		if err := t.GardenerFramework.UpdateShoot(ctx, &t.Shoot, func(shoot *gardencorev1beta1.Shoot) error {
			if err := t.GardenerFramework.GetShoot(ctx, shoot); err != nil {
				return err
			}

			shoot.Spec.Addons.NginxIngress.Enabled = true
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return applications.NewGuestBookTest(&sFramework)
}
