// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
			createShootState(fsys, "shoot")
			createControllerRegistration(fsys, "ext1")
			createControllerRegistration(fsys, "ext2")
			createControllerDeployment(fsys, "ext1")
			createControllerDeployment(fsys, "ext2")
			createExtension(fsys, "ext3")
			createExtension(fsys, "ext4")
			createConfigMap(fsys, "configmap1")
			createConfigMap(fsys, "configmap2")
			createSecret(fsys, "secret1")
			createSecret(fsys, "secret2")
			createSecretBinding(fsys, "secretBinding")
			createCredentialsBinding(fsys, "credentialsBinding")
		})

		It("should read the Kubernetes resources successfully", func() {
			resources, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).NotTo(HaveOccurred())

			Expect(resources.CloudProfile.Name).To(Equal("cpfl"))
			Expect(resources.Project.Name).To(Equal("project"))
			Expect(resources.Shoot.Name).To(Equal("shoot"))
			Expect(resources.ShootState.Name).To(Equal("shoot"))
			Expect(resources.ControllerRegistrations).To(HaveLen(4))
			Expect(resources.ControllerRegistrations[0].Name).To(Equal("ext1"))
			Expect(resources.ControllerRegistrations[1].Name).To(Equal("ext2"))
			Expect(resources.ControllerRegistrations[2].Name).To(Equal("ext3"))
			Expect(resources.ControllerRegistrations[3].Name).To(Equal("ext4"))
			Expect(resources.ControllerDeployments).To(HaveLen(4))
			Expect(resources.ControllerDeployments[0].Name).To(Equal("ext1"))
			Expect(resources.ControllerDeployments[1].Name).To(Equal("ext2"))
			Expect(resources.ControllerDeployments[2].Name).To(Equal("ext3"))
			Expect(resources.ControllerDeployments[3].Name).To(Equal("ext4"))
			Expect(resources.ConfigMaps).To(HaveLen(2))
			Expect(resources.ConfigMaps[0].Name).To(Equal("configmap1"))
			Expect(resources.ConfigMaps[1].Name).To(Equal("configmap2"))
			Expect(resources.Secrets).To(HaveLen(2))
			Expect(resources.Secrets[0].Name).To(Equal("secret1"))
			Expect(resources.Secrets[1].Name).To(Equal("secret2"))
			Expect(resources.SecretBinding.Name).To(Equal("secretBinding"))
			Expect(resources.CredentialsBinding.Name).To(Equal("credentialsBinding"))
		})

		It("should ignore hidden files", func() {
			// invalid content should not be read
			fsys[".cloudprofile-foo.yaml"] = &fstest.MapFile{Data: []byte(`{`)}

			_, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ignore hidden folders", func() {
			// invalid content should not be read
			fsys[".hidden/cloudprofile-foo.yaml"] = &fstest.MapFile{Data: []byte(`{`)}

			_, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("it cannot parse a file", func() {
		It("should return an error", func() {
			fsys["cloudprofile-foo.yaml"] = &fstest.MapFile{Data: []byte(`{`)}

			_, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).To(MatchError(ContainSubstring("failed decoding resource at index")))
		})
	})

	When("files with unexpected extension exist", func() {
		It("should return an error", func() {
			fsys["cloudprofile-foo"] = &fstest.MapFile{Data: []byte(`{"apiVersion":"core.gardener.cloud/v1beta1","kind":"CloudProfile"}`)}
			fsys["project-foo.json"] = &fstest.MapFile{Data: []byte(`{"apiVersion":"core.gardener.cloud/v1beta1","kind":"Project"}`)}
			fsys["shoot-foo.json"] = &fstest.MapFile{Data: []byte(`{"apiVersion":"core.gardener.cloud/v1beta1","kind":"Shoot"}`)}

			_, err := gardenadm.ReadManifests(log, fsys)
			Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.CloudProfile resource, but did not find any")))
		})
	})

	When("there are multiple resources of the same kind", func() {
		Describe("CloudProfile", func() {
			It("should return an error", func() {
				createCloudProfile(fsys, "obj1")
				createCloudProfile(fsys, "obj2")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.CloudProfile resource")))
			})
		})

		Describe("Project", func() {
			It("should return an error", func() {
				createProject(fsys, "obj1")
				createProject(fsys, "obj2")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.Project resource")))
			})
		})

		Describe("Shoot", func() {
			It("should return an error", func() {
				createShoot(fsys, "obj1")
				createShoot(fsys, "obj2")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.Shoot resource")))
			})
		})

		Describe("ShootState", func() {
			It("should return an error", func() {
				createShootState(fsys, "obj1")
				createShootState(fsys, "obj2")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.ShootState resource")))
			})
		})

		Describe("SecretBinding", func() {
			It("should return an error", func() {
				createSecretBinding(fsys, "secretBinding1")
				createSecretBinding(fsys, "secretBinding2")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.SecretBinding resource")))
			})
		})

		Describe("CredentialsBinding", func() {
			It("should return an error", func() {
				createCredentialsBinding(fsys, "credentialsBinding1")
				createCredentialsBinding(fsys, "credentialsBinding2")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("found more than one *securityv1alpha1.CredentialsBinding resource")))
			})
		})
	})

	When("a resource of some kind is missing", func() {
		Describe("CloudProfile", func() {
			It("should return an error", func() {
				createShoot(fsys, "shoot")
				createProject(fsys, "project")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.CloudProfile resource, but did not find any")))
			})
		})

		Describe("Project", func() {
			It("should return an error", func() {
				createCloudProfile(fsys, "cpfl")
				createShoot(fsys, "shoot")

				_, err := gardenadm.ReadManifests(log, fsys)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.Project resource, but did not find any")))
			})
		})

		Describe("Shoot", func() {
			It("should return an error", func() {
				createCloudProfile(fsys, "cpfl")
				createProject(fsys, "project")

				_, err := gardenadm.ReadManifests(log, fsys)
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

func createShootState(fsys fstest.MapFS, name string) {
	fsys["shootstate-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: ShootState
metadata:
  name: ` + name + `
`)}
}

func createControllerRegistration(fsys fstest.MapFS, name string) {
	fsys["controllerregistration-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: ` + name + `
`)}
}

func createControllerDeployment(fsys fstest.MapFS, name string) {
	fsys["controllerdeployment-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: ` + name + `
`)}
}

func createExtension(fsys fstest.MapFS, name string) {
	fsys["extension-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: operator.gardener.cloud/v1alpha1
kind: Extension
metadata:
  name: ` + name + `
spec:
  deployment:
    extension:
      helm:
        ociRepository:
          ref: garden.local/extension/` + name + `:v0.0.0
  resources:
    - kind: Infrastructure
      type: ` + name + `
    - kind: Extension
      type: ` + name + `
`)}
}

func createConfigMap(fsys fstest.MapFS, name string) {
	fsys["configmap-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: ` + name + `
`)}
}

func createSecret(fsys fstest.MapFS, name string) {
	fsys["secret-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: v1
kind: Secret
metadata:
  name: ` + name + `
`)}
}

func createSecretBinding(fsys fstest.MapFS, name string) {
	fsys["secretbinding-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: SecretBinding
metadata:
  name: ` + name + `
`)}
}

func createCredentialsBinding(fsys fstest.MapFS, name string) {
	fsys["credentialsbinding-"+name+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: security.gardener.cloud/v1alpha1
kind: CredentialsBinding
metadata:
  name: ` + name + `
`)}
}
