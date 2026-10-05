// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package restore

import (
	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

var _ = Describe("Restore-in-progress marker", func() {
	var fs afero.Afero

	BeforeEach(func() {
		fs = afero.Afero{Fs: afero.NewMemMapFs()}
	})

	Describe("#prepareRestoreInProgressMarker", func() {
		It("should write the marker and report a fresh run when it does not exist yet", func() {
			isRetry, err := prepareRestoreInProgressMarker(fs)
			Expect(err).NotTo(HaveOccurred())
			Expect(isRetry).To(BeFalse())

			exists, err := fs.Exists(cmd.RestoreInProgressLocation)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("should create the parent directory if it does not exist", func() {
			// Use a real OS-path-style fs backed by MemMapFs but without pre-creating the dir.
			emptyFs := afero.Afero{Fs: afero.NewMemMapFs()}

			isRetry, err := prepareRestoreInProgressMarker(emptyFs)
			Expect(err).NotTo(HaveOccurred())
			Expect(isRetry).To(BeFalse())

			dirExists, err := emptyFs.DirExists(filepath.Dir(cmd.RestoreInProgressLocation))
			Expect(err).NotTo(HaveOccurred())
			Expect(dirExists).To(BeTrue())
		})

		It("should report a retry and keep the marker when it already exists", func() {
			Expect(fs.WriteFile(cmd.RestoreInProgressLocation, []byte("existing-content"), 0640)).To(Succeed())

			isRetry, err := prepareRestoreInProgressMarker(fs)
			Expect(err).NotTo(HaveOccurred())
			Expect(isRetry).To(BeTrue())

			// The existing marker must not be overwritten.
			content, err := fs.ReadFile(cmd.RestoreInProgressLocation)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("existing-content"))
		})
	})

	Describe("#removeRestoreInProgressMarker", func() {
		It("should remove an existing marker", func() {
			Expect(fs.WriteFile(cmd.RestoreInProgressLocation, []byte("some-uid"), 0640)).To(Succeed())

			Expect(removeRestoreInProgressMarker(logr.Discard(), fs)).To(Succeed())

			exists, err := fs.Exists(cmd.RestoreInProgressLocation)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("should not fail when the marker does not exist", func() {
			Expect(removeRestoreInProgressMarker(logr.Discard(), fs)).To(Succeed())
		})
	})
})
