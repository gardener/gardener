// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy_test

import (
	"context"
	"io/fs"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	gardenadmbotanist "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/destroy"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/test"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Destroy", func() {
	var (
		ctx = context.Background()

		globalOpts *cmd.Options
		stdErr     *Buffer
		command    *cobra.Command

		fakeClient client.Client
		clientSet  kubernetes.Interface

		fsys fstest.MapFS
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, _, stdErr = clitest.NewTestIOStreams()
		globalOpts.Log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(stdErr))
		command = NewCommand(globalOpts)

		fakeClient = fakeclient.NewClientBuilder().Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		fsys = fstest.MapFS{
			"cloudprofile.yaml": &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
`)},
			"project.yaml": &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: garden
`)},
		}

		DeferCleanup(test.WithVars(
			&NewClientSetFromFile, func(string, *runtime.Scheme) (kubernetes.Interface, error) { return clientSet, nil },
			&gardenadmbotanist.DirFS, func(string) fs.FS { return fsys },
		))
	})

	Describe("#RunE", func() {
		BeforeEach(func() {
			Expect(command.Flags().Set("kubeconfig", "some-path-to-kubeconfig")).To(Succeed())
			Expect(command.Flags().Set("config-dir", "some-path-to-config-dir")).To(Succeed())
		})

		It("should fail if the shoot has unmanaged infrastructure", func() {
			fsys["shoot.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: root
  namespace: garden
`)}

			Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("shoot has unmanaged infrastructure")))
		})

		When("the shoot has managed infrastructure", func() {
			BeforeEach(func() {
				fsys["shoot.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: root
  namespace: garden
spec:
  credentialsBindingName: local
`)}
			})

			It("should fail if the ShootState is missing", func() {
				Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("ShootState is missing in the config directory")))
			})

			Describe("safety check", func() {
				BeforeEach(func() {
					fsys["shootstate.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: ShootState
metadata:
  name: root
  namespace: garden
`)}
				})

				It("should abort the execution if gardener-operator is deployed on the targeted cluster", func() {
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "gardener-operator"},
					})).To(Succeed())
					Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("`gardenadm destroy` does not support targeting a cluster")))
				})

				It("should abort the execution if gardenlet is deployed on the targeted cluster", func() {
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "gardenlet"},
					})).To(Succeed())
					Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring(`deployment "garden/gardenlet" exists on the targeted cluster`)))
				})
			})
		})
	})

	Describe("#Args", func() {
		It("should accept no arguments", func() {
			Expect(command.Args(command, nil)).To(Succeed())
		})

		It("should reject any positional arguments", func() {
			Expect(command.Args(command, []string{"foo"})).To(HaveOccurred())
		})
	})
})
