// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("GardenadmBotanist", func() {
	Describe("#NewGardenadmBotanistFromManifests", func() {
		const configDir = "manifests"

		var (
			ctx = context.Background()
			log = logr.Discard()

			fsys fstest.MapFS
		)

		BeforeEach(func() {
			fsys = fstest.MapFS{}
			createManifests(fsys, configDir)

			DeferCleanup(test.WithVars(
				&DirFS, func(dir string) fs.FS {
					sub, err := fs.Sub(fsys, dir)
					Expect(err).ToNot(HaveOccurred())
					return sub
				},
				&NewFs, afero.NewMemMapFs,
			))
		})

		It("should fail if the directory does not exist", func() {
			Expect(NewGardenadmBotanistFromManifests(ctx, log, nil, "does/not/exist", false)).Error().To(MatchError(fs.ErrNotExist))
		})

		When("running the control plane (acting on the self-hosted shoot cluster)", func() {
			Context("with unmanaged infrastructure", func() {
				It("should create a new Self-Hosted Botanist", func() {
					b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, true)
					Expect(err).NotTo(HaveOccurred())

					Expect(b.Shoot.CloudProfile.Name).To(Equal("stackit"))
					Expect(b.Shoot.GetInfo().Name).To(Equal("gardenadm"))
					Expect(b.Garden.Project.Name).To(Equal("gardenadm"))
					Expect(b.Extensions).To(ConsistOf(
						HaveField("ControllerRegistration.Name", "provider-stackit"),
						HaveField("ControllerRegistration.Name", "networking-cilium"),
					))
					Expect(b.Seed.GetInfo()).To(HaveField("ObjectMeta.Labels", HaveKeyWithValue("seed.gardener.cloud/self-hosted-shoot-cluster", "true")))
				})
			})

			Context("with managed infrastructure", func() {
				BeforeEach(func() {
					shootFile := fsys[configDir+"/shoot.yaml"]
					shootFile.Data = append(shootFile.Data, []byte("\n  credentialsBindingName: provider-account\n")...)

					fsys[configDir+"/shootstate.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: ShootState
metadata:
  name: gardenadm
`)}
				})

				It("should fail if the ShootState is missing", func() {
					delete(fsys, configDir+"/shootstate.yaml")
					Expect(NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, true)).Error().To(MatchError(ContainSubstring("ShootState is missing")))
				})

				It("should set the LastOperation to Restore and fetch the ShootState", func() {
					b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, true)
					Expect(err).NotTo(HaveOccurred())

					Expect(b.Shoot.GetInfo().Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
					Expect(b.Shoot.GetShootState().Name).To(Equal("gardenadm"))
					Expect(b.IsRestorePhase()).To(BeTrue())
				})
			})

			It("should use kube-system as the control plane namespace", func() {
				b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(b.Shoot.ControlPlaneNamespace).To(Equal("kube-system"))
			})

			It("should generate a UID for the shoot and write it to the host", func() {
				fs := afero.NewMemMapFs()
				DeferCleanup(test.WithVar(&NewFs, func() afero.Fs { return fs }))

				By("Generate new shoot UID and write it to the host")
				b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, true)
				Expect(err).NotTo(HaveOccurred())

				uid := b.Shoot.GetInfo().Status.UID
				Expect(uid).NotTo(BeEmpty())

				path := filepath.Join(string(filepath.Separator), "var", "lib", "gardenadm", "shoot-uid")
				content, err := b.FS.ReadFile(path)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(content)).To(Equal(string(uid)))

				By("Do not regenerate shoot UID when file is present on host")
				b, err = NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(b.Shoot.GetInfo().Status.UID).To(Equal(uid))
				content, err = b.FS.ReadFile(path)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal(string(uid)))
			})
		})

		When("not running the control plane (acting on the bootstrap cluster)", func() {
			It("should create a new Self-Hosted Botanist", func() {
				b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(b.Shoot.CloudProfile.Name).To(Equal("stackit"))
				Expect(b.Shoot.GetInfo().Name).To(Equal("gardenadm"))
				Expect(b.Garden.Project.Name).To(Equal("gardenadm"))
				Expect(b.Extensions).To(ConsistOf(
					HaveField("ControllerRegistration.Name", "provider-stackit"),
					HaveField("ControllerRegistration.Name", "dns-local"),
				))
				Expect(b.Seed.GetInfo()).To(HaveField("ObjectMeta.Labels", Not(HaveKeyWithValue("seed.gardener.cloud/self-hosted-shoot-cluster", "true"))))
			})

			It("should use the technical ID as the control plane namespace", func() {
				b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(b.Shoot.ControlPlaneNamespace).To(Equal("shoot--gardenadm--gardenadm"))
			})

			It("should generate a UID for the shoot but not write it to the host", func() {
				fs := afero.NewMemMapFs()
				DeferCleanup(test.WithVar(&NewFs, func() afero.Fs { return fs }))

				b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, false)
				Expect(err).NotTo(HaveOccurred())

				uid := b.Shoot.GetInfo().Status.UID
				Expect(uid).NotTo(BeEmpty())

				path := filepath.Join(string(filepath.Separator), "var", "lib", "gardenadm", "shoot-uid")
				Expect(b.FS.ReadFile(path)).Error().To(MatchError(os.IsNotExist, "IsNotExist"))
			})
		})

		It("should create the secrets with the fake garden client", func() {
			b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(b.GardenClient.Get(ctx, client.ObjectKey{Name: "secret1"}, &corev1.Secret{})).To(Succeed())
			Expect(b.GardenClient.Get(ctx, client.ObjectKey{Name: "secret2"}, &corev1.Secret{})).To(Succeed())
			Expect(b.GardenClient.Get(ctx, client.ObjectKey{Name: "secret-dns"}, &corev1.Secret{})).To(Succeed())
		})

		It("should create the secret binding and credentials binding", func() {
			b, err := NewGardenadmBotanistFromManifests(ctx, log, nil, configDir, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(b.GardenClient.Get(ctx, client.ObjectKey{Name: "provider-account"}, &corev1.Secret{})).To(Succeed())
			Expect(b.GardenClient.Get(ctx, client.ObjectKey{Name: "provider-account"}, &gardencorev1beta1.SecretBinding{})).To(Succeed())
			Expect(b.GardenClient.Get(ctx, client.ObjectKey{Name: "provider-account"}, &securityv1alpha1.CredentialsBinding{})).To(Succeed())
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
  dns:
    domain: api.gardenadm.local.gardener.cloud
    providers:
    - type: local
      primary: true
      secretName: secret-dns
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
  name: dns-local
spec:
  resources:
  - kind: DNSRecord
    type: local
  deployment:
    deploymentRefs:
    - name: dns-local
---
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: dns-local
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

	fsys[dir+"/secrets.yaml"] = &fstest.MapFile{Data: []byte(`---
apiVersion: v1
kind: Secret
metadata:
  name: secret1
---
apiVersion: v1
kind: Secret
metadata:
  name: secret2
---
apiVersion: v1
kind: Secret
metadata:
  name: secret-dns
---
apiVersion: v1
kind: Secret
metadata:
  name: provider-account
`)}

	fsys[dir+"/secretbinding.yaml"] = &fstest.MapFile{Data: []byte(`---
apiVersion: core.gardener.cloud/v1beta1
kind: SecretBinding
metadata:
  name: provider-account
provider:
  type: local
secretRef:
  name: provider-account
quotas: []
`)}

	fsys[dir+"/credentialsbinding.yaml"] = &fstest.MapFile{Data: []byte(`---
apiVersion: security.gardener.cloud/v1alpha1
kind: CredentialsBinding
metadata:
  name: provider-account
provider:
  type: local
credentialsRef:
  apiVersion: v1
  kind: Secret
  name: provider-account
quotas: []
`)}
}
