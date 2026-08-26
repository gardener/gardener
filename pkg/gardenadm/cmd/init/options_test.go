// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package init_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/init"
)

var _ = Describe("Options", func() {
	var (
		options   *Options
		configDir string
	)

	BeforeEach(func() {
		var err error
		configDir, err = os.MkdirTemp("", "gardenadm-test-*")
		Expect(err).NotTo(HaveOccurred())

		options = &Options{
			Options: &cmd.Options{},
		}
		options.ConfigDir = configDir

		cloudProfileManifest := `apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
spec:
  type: local
`
		Expect(os.WriteFile(filepath.Join(configDir, "cloudprofile.yaml"), []byte(cloudProfileManifest), 0644)).To(Succeed())

		projectManifest := `apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: test-project
spec:
  namespace: garden-test
`
		Expect(os.WriteFile(filepath.Join(configDir, "project.yaml"), []byte(projectManifest), 0644)).To(Succeed())

		DeferCleanup(func() {
			if configDir != "" {
				Expect(os.RemoveAll(configDir)).To(Succeed())
			}
		})
	})

	createShootManifest := func(credentialsBindingName string, zones []string, isControlPlane bool) {
		var shootManifest strings.Builder
		shootManifest.WriteString(`apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: test-shoot
  namespace: garden-test
spec:`)
		if credentialsBindingName != "" {
			shootManifest.WriteString(`
  credentialsBindingName: ` + credentialsBindingName)
		}
		shootManifest.WriteString(`
  provider:
    type: local
    workers:
    - name: control-plane
      minimum: 1
      maximum: 1`)
		if isControlPlane {
			shootManifest.WriteString(`
      controlPlane:
        highAvailability: {}`)
		}
		if len(zones) > 0 {
			shootManifest.WriteString(`
      zones:`)
			for _, zone := range zones {
				shootManifest.WriteString(`
      - ` + zone)
			}
		}
		shootManifest.WriteString(`
`)
		Expect(os.WriteFile(filepath.Join(configDir, "shoot.yaml"), []byte(shootManifest.String()), 0644)).To(Succeed())
	}

	Describe("#ParseArgs", func() {
		It("should return nil", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
		})
	})

	Describe("#Validate", func() {
		It("should fail because config dir path is not set", func() {
			options.ConfigDir = ""
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a config directory")))
		})

		It("should fail when config directory does not exist", func() {
			options.ConfigDir = "non-existent-directory"

			Expect(options.Validate()).To(MatchError(ContainSubstring("failed loading resources for zone validation")))
		})

		It("should determine the zone against the control plane worker pool", func() {
			createShootManifest("", []string{"zone-1"}, true)

			Expect(options.Validate()).To(Succeed())
			Expect(options.Zone).To(Equal("zone-1"))
		})

		It("should surface zone validation errors from the shared helper", func() {
			createShootManifest("", nil, false)

			Expect(options.Validate()).To(MatchError(ContainSubstring("shoot doesn't have a control plane worker pool configured")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
