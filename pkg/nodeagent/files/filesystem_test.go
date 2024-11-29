// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package files_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	. "github.com/gardener/gardener/pkg/nodeagent/files"
)

var _ = Describe("Filesystem", func() {
	var (
		fakeFS            afero.Afero
		fakeNodeAgentFS   *NodeAgentAfero
		testFilePath      string
		testDirectoryPath string
	)

	BeforeEach(func() {
		var err error

		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeNodeAgentFS, err = NewNodeAgentAfero(fakeFS)
		Expect(err).ToNot(HaveOccurred())

		testDirectoryPath = "/foo-directory"
		testFilePath = testDirectoryPath + "/bar-file"

		Expect(fakeFS.Mkdir(testDirectoryPath, 0700)).To(Succeed())
	})

	Describe("#GetFileSystemOperation", func() {
		It("should return `create` if the file was created by node agent fs", func() {
			_, err := fakeNodeAgentFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testFilePath)).To(Equal(OperationCreated))
		})

		It("should return `create` if a new file was written by node agent fs", func() {
			Expect(fakeNodeAgentFS.WriteFile(testFilePath, []byte("foobar"), 0600)).To(Succeed())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testFilePath)).To(Equal(OperationCreated))
		})

		It("should return `create` if the directory was created by node agent fs", func() {
			newDirectoryPath := testDirectoryPath + "/new-directory"
			Expect(fakeNodeAgentFS.Mkdir(newDirectoryPath, 0700)).To(Succeed())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(newDirectoryPath)).To(Equal(OperationCreated))
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testDirectoryPath)).To(Equal(OperationNone))
		})

		It("should return `create` if multiple directories were created by node agent fs. Existing parent directories should not be touched", func() {
			newDirectoryPath := testDirectoryPath + "/new-directory"
			newSubDirectoryPath := newDirectoryPath + "/sub-directory"
			Expect(fakeNodeAgentFS.MkdirAll(newSubDirectoryPath, 0700)).To(Succeed())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(newSubDirectoryPath)).To(Equal(OperationCreated))
			Expect(fakeNodeAgentFS.GetFileSystemOperation(newDirectoryPath)).To(Equal(OperationCreated))
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testDirectoryPath)).To(Equal(OperationNone))
		})

		It("should return `modified` if the file was modified by node agent fs", func() {
			_, err := fakeFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeNodeAgentFS.WriteFile(testFilePath, []byte("foobar"), 0600)).To(Succeed())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testFilePath)).To(Equal(OperationModified))
		})

		It("should return `deleted` if the file was deleted by node agent fs", func() {
			_, err := fakeFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeNodeAgentFS.Remove(testFilePath)).To(Succeed())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testFilePath)).To(Equal(OperationDeleted))
		})

		It("should return nothing if the file was not touched by node agent fs", func() {
			_, err := fakeFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testFilePath)).To(Equal(OperationNone))
		})

		It("should return `deleted` if the directory was deleted by node agent fs", func() {
			Expect(fakeNodeAgentFS.RemoveAll(testDirectoryPath)).To(Succeed())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testDirectoryPath)).To(Equal(OperationDeleted))
		})
	})

	Describe("#RemoveCreated", func() {
		It("should remove the file if it was created by node agent fs", func() {
			_, err := fakeNodeAgentFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeNodeAgentFS.RemoveCreated(testFilePath)).To(Succeed())
			exists, err := fakeFS.Exists(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("should not remove the file if it was not created by node agent fs", func() {
			_, err := fakeFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeNodeAgentFS.RemoveCreated(testFilePath)).To(Succeed())
			exists, err := fakeFS.Exists(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("should not remove the file if it was not created but modified by node agent fs", func() {
			_, err := fakeFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeNodeAgentFS.WriteFile(testFilePath, []byte("foobar"), 0600)).To(Succeed())
			Expect(fakeNodeAgentFS.RemoveCreated(testFilePath)).To(Succeed())
			exists, err := fakeFS.Exists(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})

	Describe("#RemoveAllCreated", func() {
		It("should remove all files and directories created by node agent fs", func() {
			newDirectoryPath := testDirectoryPath + "/new-directory"
			newSubDirectoryPath := newDirectoryPath + "/sub-directory"
			newFilePath := newSubDirectoryPath + "/new-file"
			Expect(fakeNodeAgentFS.MkdirAll(newSubDirectoryPath, 0700)).To(Succeed())
			Expect(fakeNodeAgentFS.WriteFile(newFilePath, []byte("foobar"), 0600)).To(Succeed())

			Expect(fakeNodeAgentFS.RemoveAllCreated(newDirectoryPath)).To(Succeed())

			exists, err := fakeFS.Exists(newFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(newFilePath)).To(Equal(OperationNone))

			exists, err = fakeFS.Exists(newSubDirectoryPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(newSubDirectoryPath)).To(Equal(OperationNone))

			exists, err = fakeFS.Exists(newDirectoryPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(newDirectoryPath)).To(Equal(OperationNone))

			exists, err = fakeFS.Exists(testDirectoryPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testDirectoryPath)).To(Equal(OperationNone))
		})

		It("should not remove files and directories not created by node agent fs", func() {
			_, err := fakeNodeAgentFS.Create(testFilePath)
			Expect(err).ToNot(HaveOccurred())

			nonNodeAgentFilePath := testDirectoryPath + "/non-node-agent-file"
			_, err = fakeFS.Create(nonNodeAgentFilePath)
			Expect(err).ToNot(HaveOccurred())

			nonNodeAgentDirectoryPath := testDirectoryPath + "/non-node-agent-directory"
			Expect(fakeFS.Mkdir(nonNodeAgentDirectoryPath, 0700)).To(Succeed())

			Expect(fakeNodeAgentFS.RemoveAllCreated(testDirectoryPath)).To(Succeed())

			exists, err := fakeFS.Exists(testFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testFilePath)).To(Equal(OperationNone))

			exists, err = fakeFS.Exists(nonNodeAgentFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(nonNodeAgentFilePath)).To(Equal(OperationNone))

			exists, err = fakeFS.Exists(nonNodeAgentDirectoryPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(nonNodeAgentDirectoryPath)).To(Equal(OperationNone))

			exists, err = fakeFS.Exists(testDirectoryPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(fakeNodeAgentFS.GetFileSystemOperation(testDirectoryPath)).To(Equal(OperationNone))
		})
	})
})
