// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("CloudConfigDownloaderCleaner", func() {
	var (
		ctx = context.TODO()
		log = logr.Discard()

		fakeFS   afero.Afero
		fakeDBus *fakedbus.DBus
		runnable manager.Runnable
	)

	BeforeEach(func() {
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeDBus = fakedbus.New()
		runnable = &CloudConfigDownloaderCleaner{
			Log:  log,
			FS:   fakeFS,
			DBus: fakeDBus,
		}
	})

	Describe("#Start", func() {
		var (
			pathDirectory              = filepath.Join("/", "var", "lib", "cloud-config-downloader")
			pathSystemdUnitFileSymlink = filepath.Join("/", "etc", "systemd", "system", "multi-user.target.wants", "cloud-config-downloader.service")
		)

		It("should remove the directories and files, and reload systemd daemon", func() {
			Expect(fakeFS.MkdirAll(pathDirectory, fs.ModeDir)).To(Succeed())
			_, err := fakeFS.Create(pathSystemdUnitFileSymlink)
			Expect(err).NotTo(HaveOccurred())

			Expect(runnable.Start(ctx)).To(Succeed())

			test.AssertNoDirectoryOnDisk(fakeFS, pathDirectory)
			test.AssertNoFileOnDisk(fakeFS, pathSystemdUnitFileSymlink)
			Expect(fakeDBus.Actions).To(ConsistOf(fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload}))
		})

		It("should not restart when the systemd unit file does not exist anymore", func() {
			Expect(fakeFS.MkdirAll(pathDirectory, fs.ModeDir)).To(Succeed())

			Expect(runnable.Start(ctx)).To(Succeed())

			Expect(fakeDBus.Actions).To(BeEmpty())
		})

		It("should not fail when there is nothing to cleanup", func() {
			Expect(runnable.Start(ctx)).To(Succeed())

			Expect(fakeDBus.Actions).To(BeEmpty())
		})
	})
})
