// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"io/fs"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/gardener/gardener/pkg/nodeagent/files"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("ContainerdExtractor", func() {
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
			Expect(files.Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should copy new files into a new directory", func() {
			createFile(fakeFS, sourceFile, content, 0755)
			destinationFile = path.Join(destinationFile, "more-foobar")
			Expect(files.Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should overwrite an existing file in an existing directory", func() {
			content = "foobar content: existing"
			createFile(fakeFS, destinationFile, content, 0755)
			checkFile(fakeFS, destinationFile, content, 0755)

			content = "foobar content: new"
			createFile(fakeFS, sourceFile, content, 0755)
			Expect(files.Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should copy new files into an existing directory and correct its permissions", func() {
			createFile(fakeFS, sourceFile, content, 0644)
			Expect(files.Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should overwrite an existing file with wrong permissions in an existing directory", func() {
			createFile(fakeFS, destinationFile, "permissions are 0600", 0600)

			createFile(fakeFS, sourceFile, content, 0755)
			Expect(files.Copy(fakeFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(fakeFS, destinationFile, content, permissions)
		})

		It("should not copy a source directory", func() {
			Expect(fakeFS.Mkdir(sourceFile, 0755)).To(Succeed())
			Expect(files.Copy(fakeFS, sourceFile, destinationFile, permissions)).To(MatchError(ContainSubstring("is not a regular file")))
		})

		It("should not overwrite a destination if it is a directory", func() {
			Expect(fakeFS.Mkdir(destinationFile, 0755)).To(Succeed())
			createFile(fakeFS, sourceFile, content, 0755)
			Expect(files.Copy(fakeFS, sourceFile, destinationFile, permissions)).To(MatchError(ContainSubstring("exists but is not a regular file")))
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
				Expect(files.Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should copy new files into a new directory", Offset(1), func() {
				createFile(fakeFS, sourceFile, content, permissions)
				destinationFile = path.Join(destinationFile, "more-foobar")
				Expect(files.Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should overwrite an existing file in an existing directory", Offset(1), func() {
				content = "foobar content: existing"
				createFile(fakeFS, destinationFile, content, permissions)
				checkFile(fakeFS, destinationFile, content, permissions)

				content = "foobar content: new"
				createFile(fakeFS, sourceFile, content, permissions)
				Expect(files.Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should overwrite an existing file with wrong permissions in an existing directory", Offset(1), func() {
				createFile(fakeFS, destinationFile, "permissions are 0600", 0600)

				createFile(fakeFS, sourceFile, content, permissions)
				Expect(files.Move(fakeFS, sourceFile, destinationFile)).To(Succeed())
				checkFile(fakeFS, destinationFile, content, permissions)
				checkFileNotFound(fakeFS, sourceFile)
			})

			It("should not copy a source directory", Offset(1), func() {
				Expect(fakeFS.Mkdir(sourceFile, permissions)).To(Succeed())
				Expect(files.Move(fakeFS, sourceFile, destinationFile)).To(MatchError(ContainSubstring("is not a regular file")))
			})

			It("should not overwrite a destination if it is a directory", Offset(1), func() {
				Expect(fakeFS.Mkdir(destinationFile, permissions)).To(Succeed())
				createFile(fakeFS, sourceFile, content, permissions)
				Expect(files.Move(fakeFS, sourceFile, destinationFile)).To(MatchError(ContainSubstring("exists but is not a regular file")))
			})
		}

		Context("Same device", func() { runTests() })

		Context("Cross-device", func() {
			JustBeforeEach(func() {
				DeferCleanup(test.WithVar(&files.CrossDeviceModeOnly, true))
			})

			runTests()
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
