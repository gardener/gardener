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
	defaultMrExcludeList              = ""
	defaultResourcesWithGeneratedName = ""
)

var _ = Describe("Shoot Tests", Label("Shoot"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = defaultShoot("default-")

	It("Migrate and Delete", Label("fast"), func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Migrate Shoot")
		t, err := newDefaultShootMigrationTest(parentCtx, f.Shoot)
		Expect(err).ToNot(HaveOccurred())
		beforeMigration(parentCtx, t)
		Expect(t.MigrateShoot(parentCtx)).To(Succeed())

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

func newDefaultShootMigrationTest(ctx context.Context, shoot *v1beta1.Shoot) (*ShootMigrationTest, error) {
	t, err := NewShootMigrationTest(ctx, NewGardenerFramework(defaultGardenConfig()), &ShootMigrationConfig{
		ShootName:      shoot.Name,
		ShootNamespace: shoot.Namespace,
		TargetSeedName: "local-2",
	})
	return t, err
}

func beforeMigration(ctx context.Context, t *ShootMigrationTest) error {
	if t.Shoot.Status.IsHibernated {
		return nil
	}

	By("Creating test Secret and Service Account")
	if err := t.CreateSecretAndServiceAccount(ctx); err != nil {
		Fail(err.Error())
	}

	return nil
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
	if err := t.CleanUpSecretAndServiceAccount(ctx); err != nil {
		return err
	}

	return nil
}
