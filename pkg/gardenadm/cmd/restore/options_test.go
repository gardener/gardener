// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package restore_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/restore"
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

	createShootManifest := func(credentialsBindingName string, zones []string, isControlPlane bool, statusUID string) {
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
		shootManifest.WriteString("\n")
		if statusUID != "" {
			shootManifest.WriteString(`status:
  uid: ` + statusUID + `
`)
		}

		Expect(os.WriteFile(filepath.Join(configDir, "shoot.yaml"), []byte(shootManifest.String()), 0644)).To(Succeed())
	}

	createShootStateManifest := func() {
		shootStateManifest := `apiVersion: core.gardener.cloud/v1beta1
kind: ShootState
metadata:
  name: test-shoot
  namespace: garden-test
spec:
  gardener:
  extensions:`
		Expect(os.WriteFile(filepath.Join(configDir, "shootstate.yaml"), []byte(shootStateManifest), 0644)).To(Succeed())
	}

	createBackupBucketManifest := func(name string) {
		manifest := `apiVersion: core.gardener.cloud/v1beta1
kind: BackupBucket
metadata:
  name: ` + name + `
spec:
  provider:
    type: local
    region: local
`
		Expect(os.WriteFile(filepath.Join(configDir, "backupbucket.yaml"), []byte(manifest), 0644)).To(Succeed())
	}

	createBackupEntryManifest := func(name, bucketName string) {
		manifest := `apiVersion: core.gardener.cloud/v1beta1
kind: BackupEntry
metadata:
  name: ` + name + `
  namespace: garden-test
spec:
  bucketName: ` + bucketName + `
`
		Expect(os.WriteFile(filepath.Join(configDir, "backupentry.yaml"), []byte(manifest), 0644)).To(Succeed())
	}
	Describe("#ParseArgs", func() {
		It("should return nil", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
		})
	})

	Describe("#Validate", func() {
		const (
			statusUID        = "abcd-1234-uid"
			backupBucketName = statusUID
			backupEntryName  = "kube-system--" + statusUID
		)

		BeforeEach(func() {
			createShootManifest("test-credentials", nil, true, statusUID)
			createShootStateManifest()
			createBackupBucketManifest(backupBucketName)
			createBackupEntryManifest(backupEntryName, backupBucketName)

			options.BackupDataPath = "/some/path/to/backup"
			options.PriorNodeName = "node-01"
		})

		It("should pass for valid options", func() {
			Expect(options.Validate()).To(Succeed())
		})

		It("should fail when --backup-data-path is not set", func() {
			options.BackupDataPath = ""

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide --backup-data-path")))
		})

		It("should fail when --prior-node-name is not set", func() {
			options.PriorNodeName = ""

			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide --prior-node-name")))
		})

		It("should fail when config directory does not exist", func() {
			options.ConfigDir = "non-existent-directory"

			Expect(options.Validate()).To(MatchError(ContainSubstring("failed loading resources for gardenadm restore validation")))
		})

		It("should fail when ShootState manifest is missing", func() {
			Expect(os.Remove(filepath.Join(configDir, "shootstate.yaml"))).To(Succeed())

			Expect(options.Validate()).To(MatchError(ContainSubstring("gardenadm restore requires a ShootState resource in the config directory, but none was found")))
		})

		It("should fail when Shoot .status.uid is empty", func() {
			createShootManifest("test-credentials", nil, true, "")

			Expect(options.Validate()).To(MatchError(ContainSubstring("gardenadm restore requires the Shoot manifest in the config directory to have .status.uid set")))
		})

		It("should determine the zone against the control plane worker pool", func() {
			createShootManifest("", []string{"zone-1"}, true, statusUID)

			Expect(options.Validate()).To(Succeed())
			Expect(options.Zone).To(Equal("zone-1"))
		})

		It("should surface zone validation errors from the shared helper", func() {
			createShootManifest("test-credentials", nil, false, statusUID)

			Expect(options.Validate()).To(MatchError(ContainSubstring("shoot doesn't have a control plane worker pool configured")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
