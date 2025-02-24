// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm_test

import (
	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenadm"
)

var _ = Describe("Resources", func() {
	var (
		log       = logr.Discard()
		fs        afero.Afero
		configDir string
	)

	BeforeEach(func() {
		fs = afero.Afero{Fs: afero.NewMemMapFs()}

		var err error
		configDir, err = fs.TempDir("", "testdata")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(fs.RemoveAll(configDir)).To(Succeed()) })
	})

	When("the config directory is valid", func() {
		BeforeEach(func() {
			createCloudProfile(fs, configDir, "cpfl")
			createProject(fs, configDir, "project")
			createShoot(fs, configDir, "shoot")
		})

		It("should read the Kubernetes resources successfully", func() {
			cloudProfile, project, shoot, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
			Expect(err).NotTo(HaveOccurred())

			Expect(cloudProfile).To(Equal(&gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cpfl",
				},
			}))
			Expect(project).To(Equal(&gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project",
				},
			}))
			Expect(shoot).To(Equal(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
			}))
		})
	})

	When("the config directory does not exist", func() {
		It("should return an error", func() {
			_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, "nonexistent")
			Expect(err).To(MatchError(ContainSubstring("failed walking directory")))
		})
	})

	When("it cannot parse a file", func() {
		It("should return an error", func() {
			Expect(fs.WriteFile(filepath.Join(configDir, "cloudprofile-foo.yaml"), []byte(`{`), 0600)).To(Succeed())

			_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
			Expect(err).To(MatchError(ContainSubstring("failed decoding resource at index")))
		})
	})

	When("files with unexpected extension exist", func() {
		It("should return an error", func() {
			Expect(fs.WriteFile(filepath.Join(configDir, "cloudprofile-foo"), []byte(`{}`), 0600)).To(Succeed())
			Expect(fs.WriteFile(filepath.Join(configDir, "project-foo.json"), []byte(`{}`), 0600)).To(Succeed())
			Expect(fs.WriteFile(filepath.Join(configDir, "shoot-foo.json"), []byte(`{}`), 0600)).To(Succeed())

			_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
			Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.CloudProfile resource but did not find any")))
		})
	})

	When("there are multiple resources of the same kind", func() {
		Describe("CloudProfile", func() {
			It("should return an error", func() {
				createCloudProfile(fs, configDir, "obj1")
				createCloudProfile(fs, configDir, "obj2")

				_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.CloudProfile resource")))
			})
		})

		Describe("Project", func() {
			It("should return an error", func() {
				createProject(fs, configDir, "obj1")
				createProject(fs, configDir, "obj2")

				_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.Project resource")))
			})
		})

		Describe("Shoot", func() {
			It("should return an error", func() {
				createShoot(fs, configDir, "obj1")
				createShoot(fs, configDir, "obj2")

				_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
				Expect(err).To(MatchError(ContainSubstring("found more than one *gardencorev1beta1.Shoot resource")))
			})
		})
	})

	When("a resource of some kind is missing", func() {
		Describe("CloudProfile", func() {
			It("should return an error", func() {
				createShoot(fs, configDir, "shoot")
				createProject(fs, configDir, "project")

				_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.CloudProfile resource but did not find any")))
			})
		})

		Describe("Project", func() {
			It("should return an error", func() {
				createCloudProfile(fs, configDir, "cpfl")
				createShoot(fs, configDir, "shoot")

				_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.Project resource but did not find any")))
			})
		})

		Describe("Shoot", func() {
			It("should return an error", func() {
				createCloudProfile(fs, configDir, "cpfl")
				createProject(fs, configDir, "project")

				_, _, _, err := gardenadm.ReadKubernetesResourcesFromConfigDir(log, fs, configDir)
				Expect(err).To(MatchError(ContainSubstring("must provide a *gardencorev1beta1.Shoot resource but did not find any")))
			})
		})
	})
})

func createCloudProfile(fs afero.Afero, configDir, name string) {
	ExpectWithOffset(1, fs.WriteFile(filepath.Join(configDir, "cloudprofile-"+name+".yaml"), []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: `+name+`
`), 0600)).To(Succeed())
}

func createProject(fs afero.Afero, configDir, name string) {
	ExpectWithOffset(1, fs.WriteFile(filepath.Join(configDir, "project-"+name+".yaml"), []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: `+name+`
`), 0600)).To(Succeed())
}

func createShoot(fs afero.Afero, configDir, name string) {
	ExpectWithOffset(1, fs.WriteFile(filepath.Join(configDir, "shoot-"+name+".yaml"), []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: `+name+`
`), 0600)).To(Succeed())
}
