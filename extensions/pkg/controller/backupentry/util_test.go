// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
