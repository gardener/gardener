// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
