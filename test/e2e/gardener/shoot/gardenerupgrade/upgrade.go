// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/test/e2e/gardener"
)

var (
	gardenerPreviousVersion    = os.Getenv("GARDENER_PREVIOUS_VERSION")
	gardenerPreviousGitVersion = os.Getenv("GARDENER_PREVIOUS_RELEASE")
	gardenerCurrentVersion     = os.Getenv("GARDENER_NEXT_VERSION")
	gardenerCurrentGitVersion  = os.Getenv("GARDENER_NEXT_RELEASE")

	gardenerInfoPreUpgrade  = fmt.Sprintf(" (Gardener version: %s, Git version: %s)", gardenerPreviousVersion, gardenerPreviousGitVersion)
	gardenerInfoPostUpgrade = fmt.Sprintf(" (Gardener version: %s, Git version: %s)", gardenerCurrentVersion, gardenerCurrentGitVersion)
)

func itShouldEnsureShootWasReconciledWithPreviousGardenerVersion(s *ShootContext) {
	GinkgoHelper()

	It("Ensure Shoot was reconciled with previous Gardener version", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Object(s.Shoot)).Should(HaveField("Status.Gardener.Version", Equal(gardenerPreviousVersion)))
	})
}

func itShouldEnsureShootWasReconciledWithCurrentGardenerVersion(s *ShootContext) {
	GinkgoHelper()

	It("Ensure Shoot was reconciled with current Gardener version", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Object(s.Shoot)).Should(HaveField("Status.Gardener.Version", Equal(gardenerCurrentVersion)))
	})
}
