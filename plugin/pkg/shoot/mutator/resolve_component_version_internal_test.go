// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("#resolveComponentVersion", func() {
	var profile *gardencorev1beta1.ExtensionProfile

	supported := func(version string) gardencorev1beta1.ExtensionVersion {
		return gardencorev1beta1.ExtensionVersion{
			Version:   version,
			Lifecycle: []gardencorev1beta1.LifecycleStage{{Classification: gardencorev1beta1.ClassificationSupported}},
		}
	}

	BeforeEach(func() {
		profile = &gardencorev1beta1.ExtensionProfile{}
		profile.Name = "shoot-traefik"
		profile.Spec.Components = []gardencorev1beta1.ExtensionProfileComponent{
			{
				Name: "traefik",
				Versions: []gardencorev1beta1.ExtensionVersion{
					supported("3.1.4"),
					supported("3.1.7"),
					supported("3.2.0"),
					supported("3.10.0"),
					supported("4.0.1"),
				},
			},
		}
	})

	It("should return a concrete offered version unchanged", func() {
		Expect(resolveComponentVersion(profile, "traefik", "3.1.4")).To(Equal("3.1.4"))
	})

	It("should resolve a minor-level partial to the highest patch within that minor", func() {
		Expect(resolveComponentVersion(profile, "traefik", "3.1")).To(Equal("3.1.7"))
	})

	It("should not let a minor-level partial cross into a different minor (3.1 excludes 3.10.x)", func() {
		// 3.1 means >=3.1.0 <3.2.0, so 3.10.0 must not be selected.
		Expect(resolveComponentVersion(profile, "traefik", "3.1")).NotTo(Equal("3.10.0"))
	})

	It("should resolve a major-level partial to the highest version within that major", func() {
		Expect(resolveComponentVersion(profile, "traefik", "3")).To(Equal("3.10.0"))
	})

	It("should support explicit wildcard notation", func() {
		Expect(resolveComponentVersion(profile, "traefik", "3.1.x")).To(Equal("3.1.7"))
	})

	It("should support tilde and caret ranges", func() {
		Expect(resolveComponentVersion(profile, "traefik", "~3.1")).To(Equal("3.1.7"))
		Expect(resolveComponentVersion(profile, "traefik", "^3")).To(Equal("3.10.0"))
	})

	It("should reject an unparsable version constraint", func() {
		_, err := resolveComponentVersion(profile, "traefik", "not-a-version")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("valid version constraint"))
	})

	It("should error when the component is not offered", func() {
		_, err := resolveComponentVersion(profile, "envoy", "3.1")
		Expect(err).To(MatchError(ContainSubstring("does not offer a component")))
	})

	It("should error when no supported version matches the constraint", func() {
		_, err := resolveComponentVersion(profile, "traefik", "9.9")
		Expect(err).To(MatchError(ContainSubstring("no supported version")))
	})

	It("should not resolve to a non-supported version", func() {
		profile.Spec.Components[0].Versions = []gardencorev1beta1.ExtensionVersion{
			{Version: "3.1.4", Lifecycle: []gardencorev1beta1.LifecycleStage{{Classification: gardencorev1beta1.ClassificationPreview}}},
		}

		_, err := resolveComponentVersion(profile, "traefik", "3.1")
		Expect(err).To(MatchError(ContainSubstring("no supported version")))
	})
})
