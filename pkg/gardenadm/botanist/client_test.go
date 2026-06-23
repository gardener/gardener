// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("ControlPlane", func() {
	var b *GardenadmBotanist

	BeforeEach(func() {
		b = &GardenadmBotanist{}
	})

	Describe("#DiscoverKubernetesVersion", func() {
		It("should succeed discovering the version", func() {
			version, err := b.DiscoverKubernetesVersion(fakekubernetes.NewClientSetBuilder().WithVersion("1.33.0").Build())
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(semver.MustParse("1.33.0")))
		})

		It("should fail parsing the kubernetes version", func() {
			version, err := b.DiscoverKubernetesVersion(fakekubernetes.NewClientSetBuilder().WithVersion("cannot-parse").Build())
			Expect(err).To(MatchError(ContainSubstring("failed parsing semver version")))
			Expect(version).To(BeNil())
		})
	})
})
