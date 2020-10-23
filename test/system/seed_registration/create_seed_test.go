// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		- Tests the registration of a seed
	Test: Seed registration
	Expected Output
		- Successful reconciliation after Seed registration
 **/

package seed_registration

import (
	"context"
	"time"

	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
)

const (
	CreateAndReconcileTimeout = 2 * time.Hour
)

func init() {
	framework.RegisterSeedRegistrationFrameworkFlags()
}

var _ = Describe("Seed Registration testing", func() {

	f := framework.NewSeedRegistrationFramework(&framework.SeedRegistrationConfig{
		GardenerConfig: &framework.GardenerConfig{
			CommonConfig: &framework.CommonConfig{
				ResourceDir: "../../framework/resources",
			},
		},
	})

	f.CIt("Register and Reconcile Seed", func(ctx context.Context) {
		err := f.RegisterSeed(ctx)
		framework.ExpectNoError(err)
	}, CreateAndReconcileTimeout)
})
