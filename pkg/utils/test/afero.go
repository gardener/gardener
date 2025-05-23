// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"io/fs"

	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
)

// AssertFileOnDisk asserts that a given file exists and has the expected content and mode.
func AssertFileOnDisk(fakeFS afero.Afero, path, expectedContent string, fileMode uint32) {
	description := "file path " + path

	content, err := fakeFS.ReadFile(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, string(content)).To(Equal(expectedContent), description)

	fileInfo, err := fakeFS.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(fs.FileMode(fileMode)), description)
}

// AssertNoFileOnDisk asserts that a given file does not exist.
func AssertNoFileOnDisk(fakeFS afero.Afero, path string) {
	_, err := fakeFS.ReadFile(path)
	ExpectWithOffset(1, err).To(MatchError(afero.ErrFileNotFound), "file path "+path)
}

// AssertDirectoryOnDisk asserts that a given directory exists.
func AssertDirectoryOnDisk(fakeFS afero.Afero, path string) {
	exists, err := fakeFS.DirExists(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "directory path "+path)
	ExpectWithOffset(1, exists).To(BeTrue(), "directory path "+path)
}

// AssertNoDirectoryOnDisk asserts that a given directory does not exist.
func AssertNoDirectoryOnDisk(fakeFS afero.Afero, path string) {
	exists, err := fakeFS.DirExists(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "directory path "+path)
	ExpectWithOffset(1, exists).To(BeFalse(), "directory path "+path)
}
