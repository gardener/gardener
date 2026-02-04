// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	createShootManifest := func(credentialsBindingName string, zones []string) {
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
      controlPlane:
        highAvailability: {}
      minimum: 1
      maximum: 1`)
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

			err := options.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed loading resources for zone validation"))
		})

		Context("zone validation with managed infrastructure", func() {
			BeforeEach(func() {
				createShootManifest("test-credentials", nil)
			})

			It("should reject zone when provided for managed infrastructure", func() {
				options.Zone = "us-east-1a"

				err := options.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("zone can't be configured for shoot with managed infrastrcture"))
			})

			It("should allow empty zone for managed infrastructure", func() {
				options.Zone = ""

				Expect(options.Validate()).To(Succeed())
				Expect(options.Zone).To(BeEmpty())
			})
		})

		Context("zone validation with unmanaged infrastructure", func() {
			Context("worker with no zones configured", func() {
				BeforeEach(func() {
					createShootManifest("", nil)
				})

				It("should reject zone when worker has no zones configured", func() {
					options.Zone = "custom-zone"

					err := options.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("worker \"control-plane\" has no zones configured, but zone \"custom-zone\" was provided"))
				})

				It("should allow empty zone when worker has no zones", func() {
					options.Zone = ""

					Expect(options.Validate()).To(Succeed())
					Expect(options.Zone).To(BeEmpty())
				})
			})

			Context("worker with single zone configured", func() {
				BeforeEach(func() {
					createShootManifest("", []string{"zone-1"})
				})

				It("should auto-apply the single zone when not provided", func() {
					options.Zone = ""

					Expect(options.Validate()).To(Succeed())
					Expect(options.Zone).To(Equal("zone-1"))
				})

				It("should accept matching zone when provided", func() {
					options.Zone = "zone-1"

					Expect(options.Validate()).To(Succeed())
					Expect(options.Zone).To(Equal("zone-1"))
				})

				It("should reject non-matching zone when provided", func() {
					options.Zone = "zone-2"

					err := options.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("provided zone \"zone-2\" does not match the configured zones [zone-1] for worker \"control-plane\""))
				})
			})

			Context("worker with multiple zones configured", func() {
				BeforeEach(func() {
					createShootManifest("", []string{"zone-1", "zone-2", "zone-3"})
				})

				It("should require zone flag when not provided", func() {
					options.Zone = ""

					err := options.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("worker \"control-plane\" has multiple zones configured [zone-1 zone-2 zone-3], --zone flag is required"))
				})

				It("should accept valid zone when provided", func() {
					options.Zone = "zone-2"

					Expect(options.Validate()).To(Succeed())
					Expect(options.Zone).To(Equal("zone-2"))
				})

				It("should reject invalid zone when provided", func() {
					options.Zone = "zone-4"

					err := options.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("provided zone \"zone-4\" does not match the configured zones [zone-1 zone-2 zone-3] for worker \"control-plane\""))
				})
			})
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
