// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy_test

import (
	"context"
	"errors"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
		globalOpts *cmd.Options
		stdErr     *Buffer
		command    *cobra.Command

		fakeClient      client.Client
		clientSet       kubernetes.Interface
		fakeShootClient client.Client
		shootClientSet  kubernetes.Interface
		fakeErr         = errors.New("fake")

		fsys fstest.MapFS
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, _, stdErr = clitest.NewTestIOStreams()
		globalOpts.Log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(stdErr))
		command = NewCommand(globalOpts)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		// Listing fails in the shoot cluster, so that the command stops when computing the ShootState.
		fakeShootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error { return fakeErr },
		}).Build()
		shootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeShootClient).Build()

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
			&NewClientSetFromFile, func(path string, _ *runtime.Scheme) (kubernetes.Interface, error) {
				if path == "some-path-to-shoot-kubeconfig" {
					return shootClientSet, nil
				}
				return clientSet, nil
			},
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

			When("--shoot-kubeconfig is set", func() {
				BeforeEach(func() {
					Expect(command.Flags().Set("shoot-kubeconfig", "some-path-to-shoot-kubeconfig")).To(Succeed())
				})

				It("should stop cluster-autoscaler and machine-controller-manager in the shoot and compute the ShootState", func(ctx SpecContext) {
					for _, name := range []string{"cluster-autoscaler", "machine-controller-manager"} {
						Expect(fakeShootClient.Create(ctx, &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: name},
							Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
						})).To(Succeed())
					}

					command.SetContext(ctx)
					Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("failed computing ShootState from the shoot cluster")))
					Eventually(stdErr).Should(Say("Computing ShootState from the shoot cluster"))

					for _, name := range []string{"cluster-autoscaler", "machine-controller-manager"} {
						deployment := &appsv1.Deployment{}
						Expect(fakeShootClient.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: name}, deployment)).To(Succeed())
						Expect(deployment.Spec.Replicas).To(HaveValue(BeZero()), name)
					}
				})

				It("should not fail if cluster-autoscaler and machine-controller-manager don't exist in the shoot", func(ctx SpecContext) {
					command.SetContext(ctx)
					Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("failed computing ShootState from the shoot cluster")))
				})

				It("should ignore a ShootState in the config directory", func(ctx SpecContext) {
					command.SetContext(ctx)
					fsys["shootstate.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: ShootState
metadata:
  name: root
  namespace: garden
`)}

					Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("failed computing ShootState from the shoot cluster")))
					Eventually(stdErr).Should(Say("Ignoring the ShootState in the config directory"))
				})
			})

			It("should fail if the ShootState does not belong to the Shoot", func() {
				fsys["shootstate.yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: core.gardener.cloud/v1beta1
kind: ShootState
metadata:
  name: other
  namespace: garden
`)}

				Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("ShootState garden/other does not belong to Shoot garden/root")))
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

				It("should abort the execution if gardener-operator is deployed on the targeted cluster", func(ctx SpecContext) {
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "gardener-operator"},
					})).To(Succeed())
					Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("`gardenadm destroy` does not support targeting a cluster")))
				})

				It("should abort the execution if the targeted cluster is a self-hosted shoot cluster", func(ctx SpecContext) {
					Expect(fakeClient.Create(ctx, &extensionsv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
					})).To(Succeed())
					Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("the targeted cluster is a self-hosted shoot cluster")))
				})

				It("should abort the execution if gardenlet is deployed on the targeted cluster", func(ctx SpecContext) {
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
