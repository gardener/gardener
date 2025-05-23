// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/extensions/pkg/controller/backupentry"
)

var _ = Describe("Util", func() {
	DescribeTable("#ExtractShootDetailsFromBackupEntryName",
		func(backupEntryName, expectedShootTechnicalID, expectedShootUID string) {
			shootTechnicalID, shootUID := ExtractShootDetailsFromBackupEntryName(backupEntryName)
			Expect(shootTechnicalID).To(Equal(expectedShootTechnicalID))
			Expect(shootUID).To(Equal(expectedShootUID))
		},
		Entry("with old shoot technical ID", "shoot-dev-example--f6c6fca8-9c99-11e9-829b-2a33b5079af0", "shoot-dev-example", "f6c6fca8-9c99-11e9-829b-2a33b5079af0"),
		Entry("with new shoot technical ID", "shoot--dev--example--f6c6fca8-9c99-11e9-829b-2a33b5079af0", "shoot--dev--example", "f6c6fca8-9c99-11e9-829b-2a33b5079af0"),
		Entry("without -- deliminator", "shoot-dev-example-f6c6fca8-9c99-11e9-829b-2a33b5079af0", "shoot-dev-example-f6c6fca8-9c99-11e9-829b-2a33b5079af0", "shoot-dev-example-f6c6fca8-9c99-11e9-829b-2a33b5079af0"),
		Entry("with source- prefix", "source-shoot--dev--example--f6c6fca8-9c99-11e9-829b-2a33b5079af0", "shoot--dev--example", "f6c6fca8-9c99-11e9-829b-2a33b5079af0"),
	)
})
