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

	. "github.com/gardener/gardener/pkg/nodeagent/registry"
)

var _ = Describe("ContainerdExtractor", func() {

	Describe("#CopyFile", func() {

		var (
			sourceFile      string
			destinationFile string
			content         string
		)

		BeforeEach(func() {
			var err error

			sourceDirectory, err := os.MkdirTemp("", "copy-file-sourcedir-")
			Expect(err).ToNot(HaveOccurred())
			destinationDirectory, err := os.MkdirTemp("", "copy-file-destdir-")
			Expect(err).ToNot(HaveOccurred())

			DeferCleanup(func() {
				Expect(os.RemoveAll(sourceDirectory)).To(Succeed())
				Expect(os.RemoveAll(destinationDirectory)).To(Succeed())
			})

			filename := "foobar"
			sourceFile = path.Join(sourceDirectory, filename)
			destinationFile = path.Join(destinationDirectory, filename)
			content = "foobar content"
		})

		It("should copy new files into an existing directory", func() {
			createFile(sourceFile, content, 0755)
			Expect(CopyFile(sourceFile, destinationFile)).To(Succeed())
			checkFile(destinationFile, content)
		})

		It("should copy new files into a new directory", func() {
			createFile(sourceFile, content, 0755)
			destinationFile = path.Join(destinationFile, "more-foobar")
			Expect(CopyFile(sourceFile, destinationFile)).To(Succeed())
			checkFile(destinationFile, content)
		})

		It("should overwrite an existing file in an existing directory", func() {
			content = "foobar content: existing"
			createFile(destinationFile, content, 0755)
			checkFile(destinationFile, content)

			content = "foobar content: new"
			createFile(sourceFile, content, 0755)
			Expect(CopyFile(sourceFile, destinationFile)).To(Succeed())
			checkFile(destinationFile, content)
		})

		It("should copy new files into an existing directory and correct its mode", func() {
			createFile(sourceFile, content, 0644)
			Expect(CopyFile(sourceFile, destinationFile)).To(Succeed())
			checkFile(destinationFile, content)
		})

		It("should overwrite an existing file with wrong mode in an existing directory", func() {
			createFile(destinationFile, "mode is 0600", 0600)

			createFile(sourceFile, content, 0755)
			Expect(CopyFile(sourceFile, destinationFile)).To(Succeed())
			checkFile(destinationFile, content)
		})

		It("should not copy a source directory", func() {
			Expect(os.Mkdir(sourceFile, 0755)).To(Succeed())
			Expect(CopyFile(sourceFile, destinationFile)).To(MatchError(ContainSubstring("is not a regular file")))
		})

		It("should not overwrite a destination if it is a directory", func() {
			Expect(os.Mkdir(destinationFile, 0755)).To(Succeed())
			createFile(sourceFile, content, 0755)
			Expect(CopyFile(sourceFile, destinationFile)).To(MatchError(ContainSubstring("exists but is not a regular file")))
		})
	})
})

func createFile(name, content string, mode os.FileMode) {
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	defer file.Close()
	_, err = file.WriteString(content)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
}

func checkFile(name, content string) {
	fileInfo, err := os.Stat(name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(fs.FileMode(0755)))
	file, err := os.Open(name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	defer file.Close()
	var fileContent []byte
	_, err = file.Read(fileContent)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, string(content)).To(Equal(content))
}
