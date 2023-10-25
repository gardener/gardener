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

package registry_test

import (
	"io/fs"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	. "github.com/gardener/gardener/pkg/nodeagent/registry"
)

var _ = Describe("ContainerdExtractor", func() {

	Describe("#CopyFile", func() {

		var (
			sourceFile      string
			destinationFile string
			content         string
			aferoFS         afero.Afero
			permissions     fs.FileMode
		)

		BeforeEach(func() {
			var err error

			aferoFS = afero.Afero{Fs: afero.NewMemMapFs()}
			destinationDirectory := "/copy-file-destdir"

			sourceDirectory, err := aferoFS.TempDir("", "copy-file-sourcedir-")
			Expect(err).ToNot(HaveOccurred())
			err = aferoFS.Mkdir(destinationDirectory, 0755)
			Expect(err).ToNot(HaveOccurred())

			DeferCleanup(func() {
				Expect(aferoFS.RemoveAll(sourceDirectory)).To(Succeed())
				Expect(aferoFS.RemoveAll(destinationDirectory)).To(Succeed())
			})

			filename := "foobar"
			sourceFile = path.Join(sourceDirectory, filename)
			destinationFile = path.Join(destinationDirectory, filename)
			content = "foobar content"
			permissions = 0750
		})

		It("should copy new files into an existing directory", func() {
			createFile(aferoFS, sourceFile, content, 0755)
			Expect(CopyFile(aferoFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(aferoFS, destinationFile, content, permissions)
		})

		It("should copy new files into a new directory", func() {
			createFile(aferoFS, sourceFile, content, 0755)
			destinationFile = path.Join(destinationFile, "more-foobar")
			Expect(CopyFile(aferoFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(aferoFS, destinationFile, content, permissions)
		})

		It("should overwrite an existing file in an existing directory", func() {
			content = "foobar content: existing"
			createFile(aferoFS, destinationFile, content, 0755)
			checkFile(aferoFS, destinationFile, content, 0755)

			content = "foobar content: new"
			createFile(aferoFS, sourceFile, content, 0755)
			Expect(CopyFile(aferoFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(aferoFS, destinationFile, content, permissions)
		})

		It("should copy new files into an existing directory and correct its permissions", func() {
			createFile(aferoFS, sourceFile, content, 0644)
			Expect(CopyFile(aferoFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(aferoFS, destinationFile, content, permissions)
		})

		It("should overwrite an existing file with wrong permissions in an existing directory", func() {
			createFile(aferoFS, destinationFile, "permissions are 0600", 0600)

			createFile(aferoFS, sourceFile, content, 0755)
			Expect(CopyFile(aferoFS, sourceFile, destinationFile, permissions)).To(Succeed())
			checkFile(aferoFS, destinationFile, content, permissions)
		})

		It("should not copy a source directory", func() {
			Expect(aferoFS.Mkdir(sourceFile, 0755)).To(Succeed())
			Expect(CopyFile(aferoFS, sourceFile, destinationFile, permissions)).To(MatchError(ContainSubstring("is not a regular file")))
		})

		It("should not overwrite a destination if it is a directory", func() {
			Expect(aferoFS.Mkdir(destinationFile, 0755)).To(Succeed())
			createFile(aferoFS, sourceFile, content, 0755)
			Expect(CopyFile(aferoFS, sourceFile, destinationFile, permissions)).To(MatchError(ContainSubstring("exists but is not a regular file")))
		})
	})
})

func createFile(aferoFS afero.Fs, name, content string, permissions os.FileMode) {
	file, err := aferoFS.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, permissions)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	defer file.Close()
	_, err = file.WriteString(content)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
}

func checkFile(aferoFS afero.Fs, name, content string, permissions fs.FileMode) {
	fileInfo, err := aferoFS.Stat(name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(fs.FileMode(permissions)))
	file, err := aferoFS.Open(name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	defer file.Close()
	var fileContent []byte
	_, err = file.Read(fileContent)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, string(content)).To(Equal(content))
}
