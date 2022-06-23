// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	defaultMrExcludeList              = "extension-controlplane-shoot-webhooks,extension-shoot-dns-service-shoot,extension-worker-mcm-shoot"
	defaultResourcesWithGeneratedName = "apiserver-proxy-config"
)

var _ = Describe("Shoot Tests", Label("Shoot", "control-plane-migration"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = defaultShoot("migrate-")

	It("Create, Migrate and Delete", func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Migrate Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		t, err := newDefaultShootMigrationTest(ctx, f.Shoot, f.GardenerFramework)
		Expect(err).ToNot(HaveOccurred())
		Expect(beforeMigration(ctx, t)).To(Succeed())
		Expect(t.MigrateShoot(ctx)).To(Succeed())
		Expect(afterMigration(ctx, t)).To(Succeed())

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

func newDefaultShootMigrationTest(ctx context.Context, shoot *v1beta1.Shoot, gardenerFramework *GardenerFramework) (*ShootMigrationTest, error) {
	t, err := NewShootMigrationTest(ctx, gardenerFramework, &ShootMigrationConfig{
		ShootName:               shoot.Name,
		ShootNamespace:          shoot.Namespace,
		TargetSeedName:          "local2",
		SkipNodeCheck:           true,
		SkipMachinesCheck:       true,
		SkipProtectedToleration: true,
	})
	return t, err
}

func beforeMigration(ctx context.Context, t *ShootMigrationTest) error {
	if t.Shoot.Status.IsHibernated {
		return nil
	}

	By("Creating test Secret and Service Account")
	return t.CreateSecretAndServiceAccount(ctx)
}

func afterMigration(ctx context.Context, t *ShootMigrationTest) error {
	if CurrentSpecReport().Failed() {
		t.GardenerFramework.DumpState(ctx)
		return cleanUp(ctx, t)
	}

	By("Verifying migration...")
	if err := t.VerifyMigration(ctx); err != nil {
		return err
	}

	if t.Shoot.Status.IsHibernated {
		return nil
	}

	By("Checking if the test Secret and Service Account are migrated ...")
	if err := t.CheckSecretAndServiceAccount(ctx); err != nil {
		return err
	}

	By("Checking timestamps of all resources...")
	if err := t.CheckObjectsTimestamp(ctx, strings.Split(defaultMrExcludeList, ","), strings.Split(defaultResourcesWithGeneratedName, ",")); err != nil {
		return err
	}

	return cleanUp(ctx, t)
}

func cleanUp(ctx context.Context, t *ShootMigrationTest) error {
	By("Cleaning up the test Secret and Service Account")
	return t.CleanUpSecretAndServiceAccount(ctx)
}
