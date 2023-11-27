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
