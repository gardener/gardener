// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create Shoot with Workload Identity credentials and Delete it", Label("workload-identity"), func() {
		Context("Shoot with workers", Ordered, func() {
			var s *ShootContext

			BeforeTestSetup(func() {
				shoot := DefaultShoot("e2e-wi")
				// Exercise the workload-identity credential chain (provider-local machine-controller-manager sidecar
				// and infrastructure actuator) end-to-end.
				// This shoot uses the `local-wi` CredentialsBinding backed by a WorkloadIdentity, which ships
				// via the dev-setup gardenconfig operator overlay.
				shoot.Spec.CredentialsBindingName = new("local-wi")
				s = NewTestContext().ForShoot(shoot)
			})

			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		})
	})
})
