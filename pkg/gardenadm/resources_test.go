// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm_test

import (
	"testing/fstest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenadm"
)

var _ = Describe("Resources", func() {
	var (
		log  = logr.Discard()
		fsys fstest.MapFS
	)

	BeforeEach(func() {
		fsys = fstest.MapFS{}
	})

	When("the config directory is valid", func() {
		BeforeEach(func() {
			createCloudProfile(fsys, "cpfl")
			createProject(fsys, "project")
			createShoot(fsys, "shoot")
		})

		It("should read the Kubernetes resources successfully", func() {
			cloudProfile, project, shoot, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).NotTo(HaveOccurred())

			Expect(cloudProfile.Name).To(Equal("cpfl"))
			Expect(project.Name).To(Equal("project"))
			Expect(shoot.Name).To(Equal("shoot"))
		})
	})

	When("it cannot parse a file", func() {
		It("should return an error", func() {
			fsys["cloudprofile-foo.yaml"] = &fstest.MapFile{Data: []byte(`{`)}

			_, _, _, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).To(MatchError(ContainSubstring("failed decoding resource at index")))
		})
	})

	When("files with unexpected extension exist", func() {
		It("should return an error", func() {
			fsys["cloudprofile-foo"] = &fstest.MapFile{Data: []byte(`{"apiVersion":"core.gardener.cloud/v1beta1","kind":"CloudProfile"}`)}
			fsys["project-foo.json"] = &fstest.MapFile{Data: []byte(`{"apiVersion":"core.gardener.cloud/v1beta1","kind":"Project"}`)}
			fsys["shoot-foo.json"] = &fstest.MapFile{Data: []byte(`{"apiVersion":"core.gardener.cloud/v1beta1","kind":"Shoot"}`)}

			_, _, _, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.CloudProfile resource, but did not find any")))
		})
	})

	When("there are multiple resources of the same kind", func() {
		Describe("CloudProfile", func() {
			It("should return an error", func() {
				createCloudProfile(fsys, "obj1")
				createCloudProfile(fsys, "obj2")

				_, _, _, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.CloudProfile resource")))
			})
		})

		Describe("Project", func() {
			It("should return an error", func() {
				createProject(fsys, "obj1")
				createProject(fsys, "obj2")

				_, _, _, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.Project resource")))
			})
		})

		Describe("Shoot", func() {
			It("should return an error", func() {
				createShoot(fsys, "obj1")
				createShoot(fsys, "obj2")

				_, _, _, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.Shoot resource")))
			})
		})
	})

	When("a resource of some kind is missing", func() {
		Describe("CloudProfile", func() {
			It("should return an error", func() {
				createShoot(fsys, "shoot")
				createProject(fsys, "project")

				_, _, _, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.CloudProfile resource, but did not find any")))
			})
		})

		Describe("Project", func() {
			It("should return an error", func() {
				createCloudProfile(fsys, "cpfl")
				createShoot(fsys, "shoot")

				_, _, _, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.Project resource, but did not find any")))
			})
		})

		Describe("Shoot", func() {
			It("should return an error", func() {
				createCloudProfile(fsys, "cpfl")
				createProject(fsys, "project")

				_, _, _, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.Shoot resource, but did not find any")))
			})
		})
	})
})

func createCloudProfile(fsys fstest.MapFS, name string) {
	fsys["cloudprofile-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: ` + name + `
`)}
}

func createProject(fsys fstest.MapFS, name string) {
	fsys["project-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: ` + name + `
`)}
}

func createShoot(fsys fstest.MapFS, name string) {
	fsys["shoot-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: ` + name + `
`)}
}
