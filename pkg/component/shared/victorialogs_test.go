// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/component/shared"
)

var _ = Describe("VictoriaLogs", func() {
	Describe("#SplitImageRef", func() {
		const (
			repository = "europe-docker.pkg.dev/gardener-project/releases/victoria-logs"
			digest     = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
		)

		DescribeTable("should split the image reference into repository and tag",
			func(image, expectedRepository, expectedTag string) {
				gotRepository, gotTag, err := SplitImageRef(image)
				Expect(err).NotTo(HaveOccurred())
				Expect(gotRepository).To(Equal(expectedRepository))
				Expect(gotTag).To(Equal(expectedTag))
			},

			Entry("ref form (repository and tag)",
				repository+":v1.2.3",
				repository,
				"v1.2.3",
			),
			Entry("repository and digest",
				repository+"@"+digest,
				repository,
				digest,
			),
			Entry("tag and digest combined",
				repository+":v1.2.3@"+digest,
				repository,
				"v1.2.3@"+digest,
			),
		)

		It("should return an error for an invalid image reference", func() {
			_, _, err := SplitImageRef("::invalid::")
			Expect(err).To(HaveOccurred())
		})
	})
})
