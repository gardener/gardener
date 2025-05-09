// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"io/fs"
	"testing/fstest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("AutonomousBotanist", func() {
	Describe("#NewAutonomousBotanistFromManifests", func() {
		const configDir = "manifests"

		var (
			ctx = context.Background()
			log = logr.Discard()

			fsys fstest.MapFS
		)

		BeforeEach(func() {
			fsys = fstest.MapFS{}
			createManifests(fsys, configDir)

			DeferCleanup(test.WithVar(&DirFS, func(dir string) fs.FS {
				sub, err := fs.Sub(fsys, dir)
				Expect(err).ToNot(HaveOccurred())
				return sub
			}))
		})

		It("should fail if the directory does not exist", func() {
			Expect(NewAutonomousBotanistFromManifests(ctx, log, nil, "does/not/exist")).Error().To(MatchError(fs.ErrNotExist))
		})

		It("should create a new Autonomous Botanist", func() {
			b, err := NewAutonomousBotanistFromManifests(ctx, log, nil, configDir)
			Expect(err).NotTo(HaveOccurred())

			Expect(b.Shoot.CloudProfile.Name).To(Equal("stackit"))
			Expect(b.Shoot.GetInfo().Name).To(Equal("gardenadm"))
			Expect(b.Garden.Project.Name).To(Equal("gardenadm"))
			Expect(b.Extensions).To(ConsistOf(
				HaveField("ControllerRegistration.Name", "provider-stackit"),
				HaveField("ControllerRegistration.Name", "networking-cilium"),
			))
		})
	})
})

func createManifests(fsys fstest.MapFS, dir string) {
	fsys[dir+"/cloudprofile.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: stackit
`)}

	fsys[dir+"/project.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: gardenadm
`)}

	fsys[dir+"/shoot.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: gardenadm
spec:
  kubernetes:
    version: "1.33"
  provider:
    type: stackit
    workers:
    - name: control-plane
      controlPlane: {}
  networking:
    type: cilium
    nodes: 10.1.0.0/16
    pods: 10.3.0.0/16
    services: 10.4.0.0/16
`)}

	fsys[dir+"/extensions.yaml"] = &fstest.MapFile{Data: []byte(`---
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: provider-stackit
spec:
  resources:
  - kind: ControlPlane
    type: stackit
  - kind: Infrastructure
    type: stackit
  - kind: Worker
    type: stackit
  deployment:
    deploymentRefs:
    - name: provider-stackit
---
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: provider-stackit
---
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: networking-cilium
spec:
  resources:
  - kind: Network
    type: cilium
  deployment:
    deploymentRefs:
    - name: networking-cilium
---
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: networking-cilium
---
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: unused
spec:
  resources:
  - kind: Network
    type: unused
  deployment:
    deploymentRefs:
    - name: unused
---
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: unused
`)}
}
