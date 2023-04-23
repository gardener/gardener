// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/framework"
)

var _ = Describe("Shoot Tests", Label("Shoot", "control-plane-migration"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = e2e.DefaultShoot("e2e-migrate")
	// Assign seedName so that shoot does not get scheduled to the seed that will be used as target.
	f.Shoot.Spec.SeedName = pointer.String(getSeedName(false))

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
		Expect(t.MigrateShoot(ctx)).To(Succeed())
		Expect(t.VerifyMigration(ctx)).To(Succeed())

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

func newDefaultShootMigrationTest(ctx context.Context, shoot *v1beta1.Shoot, gardenerFramework *GardenerFramework) (*ShootMigrationTest, error) {
	t, err := NewShootMigrationTest(ctx, gardenerFramework, &ShootMigrationConfig{
		ShootName:               shoot.Name,
		ShootNamespace:          shoot.Namespace,
		TargetSeedName:          getSeedName(true),
		SkipShootClientCreation: true,
		SkipNodeCheck:           true,
		SkipMachinesCheck:       true,
		SkipProtectedToleration: true,
	})
	return t, err
}

func getSeedName(isTarget bool) (seedName string) {
	switch os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE") {
	case "node":
		seedName = "local-ha-single-zone"
		if isTarget {
			seedName = "local2-ha-single-zone"
		}
	default:
		seedName = "local"
		if isTarget {
			seedName = "local2"
		}
	}

	return
}
