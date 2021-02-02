// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		- Tests the creation of a shoot

	BeforeSuite
		- Parse Shoot from example folder and provided flags

	Test: Shoot creation
	Expected Output
		- Successful reconciliation after Shoot creation
 **/

package shoot_creation

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	CreateAndReconcileTimeout = 2 * time.Hour
)

func init() {
	framework.RegisterShootCreationFrameworkFlags()
}

var _ = Describe("Shoot Creation testing", func() {

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: &framework.GardenerConfig{
			CommonConfig: &framework.CommonConfig{
				ResourceDir: "../../framework/resources",
			},
		},
	})

	f.CIt("Create and Reconcile Shoot", func(ctx context.Context) {
		actual, err := f.CreateShoot(ctx, true, true)
		framework.ExpectNoError(err)

		// Verify Shoot status
		var (
			expectedTechnicalID           = fmt.Sprintf("shoot--%s--%s", f.GetShootFramework().Project.Name, actual.Name)
			expectedClusterIdentityPrefix = fmt.Sprintf("%s-%s", actual.Status.TechnicalID, actual.Status.UID)
		)

		Expect(actual.Status.Gardener.ID).NotTo(BeEmpty())
		Expect(actual.Status.Gardener.Name).NotTo(BeEmpty())
		Expect(actual.Status.Gardener.Version).NotTo(BeEmpty())
		Expect(actual.Status.LastErrors).To(BeEmpty())
		Expect(actual.Status.SeedName).NotTo(BeNil())
		Expect(*actual.Status.SeedName).NotTo(BeEmpty())
		Expect(actual.Status.TechnicalID).To(Equal(expectedTechnicalID))
		Expect(actual.Status.UID).NotTo(BeEmpty())
		Expect(actual.Status.ClusterIdentity).NotTo(BeNil())
		Expect(*actual.Status.ClusterIdentity).To(HavePrefix(expectedClusterIdentityPrefix))
	}, CreateAndReconcileTimeout)
})
