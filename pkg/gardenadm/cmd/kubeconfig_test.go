// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd"
)

var _ = Describe("Kubeconfig", func() {
	Describe("#DefaultKubeconfig", func() {
		It("should return an error if the kubeconfig pointer is nil", func() {
			Expect(DefaultKubeconfig(nil)).To(MatchError("kubeconfig pointer must not be nil"))
		})

		When("kubeconfig pointer is set", func() {
			var kubeconfig *string

			BeforeEach(func() {
				kubeconfig = ptr.To("")
			})

			It("should do nothing when the value is already set", func() {
				*kubeconfig = "foo"

				Expect(DefaultKubeconfig(kubeconfig)).To(Succeed())
				Expect(kubeconfig).To(gstruct.PointTo(Equal("foo")))
			})

			It("should use the KUBECONFIG env variable when set", func() {
				os.Setenv("KUBECONFIG", "foo")
				DeferCleanup(func() { os.Setenv("KUBECONFIG", "") })

				Expect(DefaultKubeconfig(kubeconfig)).To(Succeed())
				Expect(kubeconfig).To(gstruct.PointTo(Equal("foo")))
			})

			It("should use the default kubeconfig location from the home dir", func() {
				homeDir, err := os.UserHomeDir()
				Expect(err).NotTo(HaveOccurred())

				Expect(DefaultKubeconfig(kubeconfig)).To(Succeed())
				Expect(kubeconfig).To(gstruct.PointTo(Equal(filepath.Join(homeDir, ".kube", "config"))))
			})
		})
	})
})
