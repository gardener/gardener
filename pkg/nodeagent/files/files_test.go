// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package files_test

import (
	"io/fs"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	. "github.com/gardener/gardener/pkg/nodeagent/files"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Files", func() {
	Describe("#Copy", func() {
		var (
			sourceFile      string
			destinationFile string
			content         string
			fakeFS          afero.Afero
			permissions     fs.FileMode
		)

		BeforeEach(func() {
			var err error

			fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
			destinationDirectory := "/copy-file-destdir"

			sourceDirectory, err := fakeFS.TempDir("", "copy-file-sourcedir-")
			Expect(err).ToNot(HaveOccurred())
			err = fakeFS.Mkdir(destinationDirectory, 0755)
			Expect(err).ToNot(HaveOccurred())

			DeferCleanup(func() {
				Expect(fakeFS.RemoveAll(sourceDirectory)).To(Succeed())
				Expect(fakeFS.RemoveAll(destinationDirectory)).To(Succeed())
			})

			filename := "foobar"
			sourceFile = path.Join(sourceDirectory, filename)
			destinationFile = path.Join(destinationDirectory, filename)
			content = "foobar content"
			permissions = 0750
		})

		It("should copy new files into an existing directory", func() {
			createFile(fakeFS, sourceFile, content, 0755)
			Expect(Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should copy new files into a new directory", func() {
			createFile(fakeFS, sourceFile, content, 0755)
			destinationFile = path.Join(destinationFile, "more-foobar")
			Expect(Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should overwrite an existing file in an existing directory", func() {
			content = "foobar content: existing"
			createFile(fakeFS, destinationFile, content, 0755)
			checkFile(fakeFS, destinationFile, content, 0755)

			content = "foobar content: new"
			createFile(fakeFS, sourceFile, content, 0755)
			Expect(Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should copy new files into an existing directory and correct its permissions", func() {
			createFile(fakeFS, sourceFile, content, 0644)
			Expect(Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should overwrite an existing file with wrong permissions in an existing directory", func() {
			createFile(fakeFS, destinationFile, "permissions are 0600", 0600)

			createFile(fakeFS, sourceFile, content, 0755)
			Expect(Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should not copy a source directory", func() {
			Expect(fakeFS.Mkdir(sourceFile, 0755)).To(Succeed())
			Expect(Copy(fakeFS, sourceFile, destinationFile, permissions)).To(MatchError(ContainSubstring("is not a regular file")))
		})

		It("should not overwrite a destination if it is a directory", func() {
			Expect(fakeFS.Mkdir(destinationFile, 0755)).To(Succeed())
			createFile(fakeFS, sourceFile, content, 0755)
			Expect(Copy(fakeFS, sourceFile, destinationFile, permissions)).To(MatchError(ContainSubstring("exists but is not a regular file")))
		})
	})

	Describe("#Move", func() {
		var (
			sourceFile      string
			destinationFile string
			content         string
			fakeFS          afero.Afero
			permissions     fs.FileMode
		)

		BeforeEach(func() {
			var err error

			fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
			destinationDirectory := "/move-file-destdir"

			sourceDirectory, err := fakeFS.TempDir("", "move-file-sourcedir-")
			Expect(err).ToNot(HaveOccurred())
			err = fakeFS.Mkdir(destinationDirectory, 0755)
			Expect(err).ToNot(HaveOccurred())

			DeferCleanup(func() {
				Expect(fakeFS.RemoveAll(sourceDirectory)).To(Succeed())
				Expect(fakeFS.RemoveAll(destinationDirectory)).To(Succeed())
			})

			filename := "foobar"
			sourceFile = path.Join(sourceDirectory, filename)
			destinationFile = path.Join(destinationDirectory, filename)
			content = "foobar content"
			permissions = 0750
		})

		runTests := func() {
			It("should move new files into an existing directory", Offset(1), func() {
				createFile(fakeFS, sourceFile, content, permissions)
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should copy new files into a new directory", Offset(1), func() {
				createFile(fakeFS, sourceFile, content, permissions)
				destinationFile = path.Join(destinationFile, "more-foobar")
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should overwrite an existing file in an existing directory", Offset(1), func() {
				content = "foobar content: existing"
				createFile(fakeFS, destinationFile, content, permissions)
				checkFile(fakeFS, destinationFile, content, permissions)

				content = "foobar content: new"
				createFile(fakeFS, sourceFile, content, permissions)
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should overwrite an existing file with wrong permissions in an existing directory", Offset(1), func() {
				createFile(fakeFS, destinationFile, "permissions are 0600", 0600)

				createFile(fakeFS, sourceFile, content, permissions)
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should not copy a source directory", Offset(1), func() {
				Expect(fakeFS.Mkdir(sourceFile, permissions)).To(Succeed())
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(MatchError(ContainSubstring("is not a regular file")))
			})

			It("should not overwrite a destination if it is a directory", Offset(1), func() {
				Expect(fakeFS.Mkdir(destinationFile, permissions)).To(Succeed())
				createFile(fakeFS, sourceFile, content, permissions)
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(MatchError(ContainSubstring("exists but is not a regular file")))
			})
		}

		Context("Same device", func() { runTests() })

		Context("Cross-device", func() {
			JustBeforeEach(func() {
				DeferCleanup(test.WithVar(&CrossDeviceModeOnly, true))
			})

			runTests()

			It("should work if a tmp file from a previous run still exists", func() {
				createFile(fakeFS, destinationFile+".tmp", content, permissions)
				createFile(fakeFS, sourceFile, content, permissions)
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
				checkFileNotFound(fakeFS, destinationFile+".tmp")
			})

			It("should not delete if there .tmp file exists and is a directory", func() {
				Expect(fakeFS.Mkdir(destinationFile, permissions)).To(Succeed())
				createFile(fakeFS, sourceFile, content, permissions)
				Expect(Move(fakeFS, sourceFile, destinationFile)).To(MatchError(ContainSubstring("exists but is not a regular file")))
			})
		})
	})
})

func createFile(fakeFS afero.Fs, name, content string, permissions os.FileMode) {
	file, err := fakeFS.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, permissions)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	defer func(f afero.File) { Expect(f.Close()).To(Succeed()) }(file)

	_, err = file.WriteString(content)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
}

func checkFile(fakeFS afero.Fs, name, content string, permissions fs.FileMode) {
	fileInfo, err := fakeFS.Stat(name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(permissions))

	file, err := fakeFS.Open(name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	defer func(f afero.File) { Expect(f.Close()).To(Succeed()) }(file)

	var fileContent []byte
	_, err = file.Read(fileContent)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, content).To(Equal(content))
}

func checkFileNotFound(fakeFS afero.Fs, name string) {
	_, err := fakeFS.Stat(name)
	ExpectWithOffset(1, err).To(MatchError(ContainSubstring("file does not exist")))
}
