// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("ControlPlane", func() {
	var b *AutonomousBotanist

	BeforeEach(func() {
		b = &AutonomousBotanist{}
	})

	Describe("#DiscoverKubernetesVersion", func() {
		var (
			controlPlaneAddress = "control-plane-address"
			token               = "token"
			caBundle            = []byte("ca-bundle")
		)

		It("should succeed discovering the version", func() {
			DeferCleanup(test.WithVar(&NewWithConfig, func(_ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return fakekubernetes.NewClientSetBuilder().WithVersion("1.33.0").Build(), nil
			}))

			version, err := b.DiscoverKubernetesVersion(controlPlaneAddress, caBundle, token)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(semver.MustParse("1.33.0")))
		})

		It("should fail creating the client set from the kubeconfig", func() {
			DeferCleanup(test.WithVar(&NewWithConfig, func(_ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return nil, fmt.Errorf("fake err")
			}))

			version, err := b.DiscoverKubernetesVersion(controlPlaneAddress, caBundle, token)
			Expect(err).To(MatchError(ContainSubstring("fake err")))
			Expect(version).To(BeNil())
		})

		It("should fail parsing the kubernetes version", func() {
			DeferCleanup(test.WithVar(&NewWithConfig, func(_ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return fakekubernetes.NewClientSetBuilder().WithVersion("cannot-parse").Build(), nil
			}))

			version, err := b.DiscoverKubernetesVersion(controlPlaneAddress, caBundle, token)
			Expect(err).To(MatchError(ContainSubstring("failed parsing semver version")))
			Expect(version).To(BeNil())
		})
	})
})
