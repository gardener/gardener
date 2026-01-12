// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discover_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/discover"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/test"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Discover", func() {
	var (
		globalOpts     *cmd.Options
		stdOut, stdErr *Buffer
		command        *cobra.Command

		fs         afero.Afero
		fakeClient client.Client
		clientSet  kubernetes.Interface
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, stdOut, stdErr = clitest.NewTestIOStreams()
		globalOpts.Log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(stdErr))
		command = NewCommand(globalOpts)

		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.Project{}, core.ProjectNamespace, indexer.ProjectNamespaceIndexerFunc).
			Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		fs = afero.Afero{Fs: afero.NewMemMapFs()}

		DeferCleanup(test.WithVars(
			&NewClientSetFromFile, func(string, *runtime.Scheme) (kubernetes.Interface, error) { return clientSet, nil },
			&NewAferoFs, func() afero.Afero { return fs },
		))
	})

	Describe("#RunE", func() {
		var (
			ctx                   = context.Background()
			namespaceName         = "garden-test-project"
			extensionTypeProvider = "test-extension-type-provider"
			extensionTypeNetwork  = "test-extension-type-network"
			extensionTypeDNS      = "test-extension-type-dns"

			project                        *gardencorev1beta1.Project
			namespace                      *corev1.Namespace
			secretBinding                  *gardencorev1beta1.SecretBinding
			secret, secretDNS              *corev1.Secret
			cloudProfile                   *gardencorev1beta1.CloudProfile
			controllerDeploymentProvider   *gardencorev1.ControllerDeployment
			controllerRegistrationProvider *gardencorev1beta1.ControllerRegistration
			controllerDeploymentNetwork    *gardencorev1.ControllerDeployment
			controllerRegistrationNetwork  *gardencorev1beta1.ControllerRegistration
			controllerDeploymentDNS        *gardencorev1.ControllerDeployment
			controllerRegistrationDNS      *gardencorev1beta1.ControllerRegistration

			shoot             *gardencorev1beta1.Shoot
			shootRaw          []byte
			shootManifestPath = "some-path-to-shoot-manifest-file"
		)

		BeforeEach(func() {
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: &namespaceName,
				},
			}
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: namespaceName,
				},
			}
			secretDNS = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-dns",
					Namespace: namespaceName,
				},
			}
			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-binding",
					Namespace: namespaceName,
				},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}
			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cloud-profile",
				},
			}
			controllerDeploymentProvider = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-controller-deployment-provider",
				},
			}
			controllerRegistrationProvider = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-controller-registration-provider",
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "ControlPlane", Type: extensionTypeProvider},
						{Kind: "Infrastructure", Type: extensionTypeProvider},
						{Kind: "Worker", Type: extensionTypeProvider},
					},
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: controllerDeploymentProvider.Name}},
					},
				},
			}
			controllerDeploymentNetwork = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-controller-deployment-network",
				},
			}
			controllerRegistrationNetwork = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-controller-registration-network",
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "Network", Type: extensionTypeNetwork},
					},
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: controllerDeploymentNetwork.Name}},
					},
				},
			}
			controllerDeploymentDNS = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-controller-deployment-dns",
				},
			}
			controllerRegistrationDNS = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-controller-registration-dns",
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "DNSRecord", Type: extensionTypeDNS},
					},
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: controllerDeploymentDNS.Name}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, project)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretDNS)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeClient.Create(ctx, controllerDeploymentProvider)).To(Succeed())
			Expect(fakeClient.Create(ctx, controllerRegistrationProvider)).To(Succeed())
			Expect(fakeClient.Create(ctx, controllerDeploymentNetwork)).To(Succeed())
			Expect(fakeClient.Create(ctx, controllerRegistrationNetwork)).To(Succeed())
			Expect(fakeClient.Create(ctx, controllerDeploymentDNS)).To(Succeed())
			Expect(fakeClient.Create(ctx, controllerRegistrationDNS)).To(Succeed())

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: namespaceName,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: &secretBinding.Name,
					CloudProfile: &gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: cloudProfile.Name,
					},
					Provider: gardencorev1beta1.Provider{
						Type:    extensionTypeProvider,
						Workers: []gardencorev1beta1.Worker{{}},
					},
					Networking: &gardencorev1beta1.Networking{
						Type: &extensionTypeNetwork,
					},
					DNS: &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{
							{
								Type:    ptr.To(extensionTypeDNS),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       secretDNS.Name,
								},
							},
							{
								Type: ptr.To("unused"),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-credentials-unused",
								},
							},
						},
					},
				},
			}

			var err error
			shootRaw, err = runtime.Encode(&json.Serializer{}, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(fs.WriteFile(shootManifestPath, shootRaw, 0600)).To(Succeed())
		})

		It("should return the expected output", func() {
			Expect(command.Flags().Set("kubeconfig", "some-path-to-kubeconfig")).To(Succeed())
			Expect(command.RunE(command, []string{shootManifestPath})).To(Succeed())

			Eventually(func() string { return string(stdOut.Contents()) }).Should(SatisfyAll(
				ContainSubstring("Computing required resources for Shoot..."),
				ContainSubstring("Fetching required resources for from garden cluster..."),
				ContainSubstring("Exported Namespace/"+namespace.Name),
				ContainSubstring("Exported Project/"+project.Name),
				ContainSubstring("Exported Secret/"+secret.Name),
				ContainSubstring("Exported Secret/"+secretDNS.Name),
				ContainSubstring("Exported SecretBinding/"+secretBinding.Name),
				ContainSubstring("Exported CloudProfile/"+cloudProfile.Name),
				ContainSubstring("Exported ControllerDeployment/"+controllerDeploymentProvider.Name),
				ContainSubstring("Exported ControllerRegistration/"+controllerRegistrationProvider.Name),
				ContainSubstring("Exported ControllerDeployment/"+controllerDeploymentNetwork.Name),
				ContainSubstring("Exported ControllerRegistration/"+controllerRegistrationNetwork.Name),
				ContainSubstring("Exported ControllerDeployment/"+controllerDeploymentDNS.Name),
				ContainSubstring("Exported ControllerRegistration/"+controllerRegistrationDNS.Name),
			))

			for _, path := range []string{
				fmt.Sprintf("namespace-%s.yaml", namespace.Name),
				fmt.Sprintf("project-%s.yaml", project.Name),
				fmt.Sprintf("secret-%s.yaml", secret.Name),
				fmt.Sprintf("secret-%s.yaml", secretDNS.Name),
				fmt.Sprintf("secretbinding-%s.yaml", secretBinding.Name),
				fmt.Sprintf("cloudprofile-%s.yaml", cloudProfile.Name),
				fmt.Sprintf("controllerdeployment-%s.yaml", controllerDeploymentProvider.Name),
				fmt.Sprintf("controllerregistration-%s.yaml", controllerRegistrationProvider.Name),
				fmt.Sprintf("controllerdeployment-%s.yaml", controllerDeploymentNetwork.Name),
				fmt.Sprintf("controllerregistration-%s.yaml", controllerRegistrationNetwork.Name),
				fmt.Sprintf("controllerdeployment-%s.yaml", controllerDeploymentDNS.Name),
				fmt.Sprintf("controllerregistration-%s.yaml", controllerRegistrationDNS.Name),
			} {
				exists, err := fs.Exists(path)
				Expect(err).NotTo(HaveOccurred(), "for path "+path)
				Expect(exists).To(BeTrue(), "for path "+path)
			}
		})
	})
})
