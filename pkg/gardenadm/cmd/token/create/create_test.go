// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package create_test

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/create"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Create", func() {
	var (
		ctx = context.Background()

		globalOpts *cmd.Options
		stdOut     *Buffer
		command    *cobra.Command

		fakeClient client.Client
		clientSet  kubernetes.Interface
		restConfig = &rest.Config{Host: "some-host", TLSClientConfig: rest.TLSClientConfig{CAData: []byte("ca-data")}}

		tokenID     = "abcdef"
		tokenSecret = "1234567890abcdef"
		token       = tokenID + "." + tokenSecret
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, stdOut, _ = clitest.NewTestIOStreams()
		command = NewCommand(globalOpts)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).WithRESTConfig(restConfig).Build()

		DeferCleanup(test.WithVar(&tokenutils.CreateClientSet, func(context.Context, logr.Logger) (kubernetes.Interface, error) { return clientSet, nil }))
	})

	Describe("#RunE", func() {
		It("should compute a random bootstrap token and print it", func() {
			Expect(command.RunE(command, nil)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList)).To(Succeed())

			Expect(secretList.Items).To(HaveLen(1))
			secret := secretList.Items[0]
			Expect(secret.Data).To(And(
				HaveKeyWithValue("token-id", Not(BeEmpty())),
				HaveKeyWithValue("token-secret", Not(BeEmpty())),
			))

			Eventually(stdOut).Should(Say(string(secret.Data["token-id"]) + "." + string(secret.Data["token-secret"])))
		})

		It("should create the specified bootstrap token and print it", func() {
			Expect(command.RunE(command, []string{token})).To(Succeed())

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-" + tokenID, Namespace: "kube-system"}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

			Expect(secret.Data).To(And(
				HaveKeyWithValue("token-id", Equal([]byte(tokenID))),
				HaveKeyWithValue("token-secret", Equal([]byte(tokenSecret))),
			))

			Eventually(stdOut).Should(Say(token))
		})

		It("should return an error because a bootstrap token with the ID already exists", func() {
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-" + tokenID, Namespace: "kube-system"}})).To(Succeed())

			Expect(command.RunE(command, []string{token})).To(MatchError(ContainSubstring("already exists")))
		})

		Context("join and reset command", func() {
			var shoot *gardencorev1beta1.Shoot

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					TypeMeta: metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Shoot"},
					Spec: gardencorev1beta1.ShootSpec{
						CredentialsBindingName: new("credentials"),
						DNS:                    &gardencorev1beta1.DNS{Domain: new("root.example.com")},
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{
								Name:         "control-plane",
								ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
							}},
						},
					},
				}
			})

			createShootInfoAndCluster := func() {
				GinkgoHelper()

				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ConfigMapNameShootInfo, Namespace: metav1.NamespaceSystem}})).To(Succeed())

				shootRaw, err := json.Marshal(shoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeClient.Create(ctx, &extensionsv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem},
					Spec:       extensionsv1alpha1.ClusterSpec{Shoot: runtime.RawExtension{Raw: shootRaw}},
				})).To(Succeed())
			}

			When("the join command should be printed", func() {
				BeforeEach(func() {
					Expect(command.Flags().Set("print-join-command", "true")).To(Succeed())
				})

				It("should return an error because the cluster is not a Shoot", func() {
					Expect(command.RunE(command, []string{token})).To(MatchError(ContainSubstring("`--print-{join,reset}-command` can only be used when the kubeconfig points to a Shoot cluster")))
				})

				It("should return an error because the Cluster resource does not exist", func() {
					Expect(fakeClient.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ConfigMapNameShootInfo, Namespace: metav1.NamespaceSystem}})).To(Succeed())

					Expect(command.RunE(command, []string{token})).To(MatchError(ContainSubstring("failed reading Shoot from Cluster resource")))
				})

				It("should successfully print the join command", func() {
					createShootInfoAndCluster()

					Expect(command.RunE(command, []string{token})).To(Succeed())
					Eventually(stdOut).Should(Say(`gardenadm join --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" some-host
`))
				})

				It("should print the join command with the internal domain if the control plane is exposed", func() {
					shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = &gardencorev1beta1.Exposure{DNS: &gardencorev1beta1.DNSExposure{}}
					createShootInfoAndCluster()

					Expect(command.RunE(command, []string{token})).To(Succeed())
					Eventually(stdOut).Should(Say(`gardenadm join --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" https://api.internal.root.example.com
`))
				})
			})

			When("the reset command should be printed", func() {
				BeforeEach(func() {
					Expect(command.Flags().Set("print-reset-command", "true")).To(Succeed())
				})

				It("should return an error because the cluster is not a Shoot", func() {
					Expect(command.RunE(command, []string{token})).To(MatchError(ContainSubstring("`--print-{join,reset}-command` can only be used when the kubeconfig points to a Shoot cluster")))
				})

				It("should successfully print the reset command", func() {
					createShootInfoAndCluster()

					Expect(command.RunE(command, []string{token})).To(Succeed())
					Eventually(stdOut).Should(Say(`gardenadm reset --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" some-host
`))
				})

				It("should print the reset command with the internal domain if the control plane is exposed", func() {
					shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = &gardencorev1beta1.Exposure{Extension: &gardencorev1beta1.ExtensionExposure{}}
					createShootInfoAndCluster()

					Expect(command.RunE(command, []string{token})).To(Succeed())
					Eventually(stdOut).Should(Say(`gardenadm reset --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" https://api.internal.root.example.com
`))
				})
			})
		})

		When("the connect command should be printed", func() {
			BeforeEach(func() {
				Expect(command.Flags().Set("print-connect-command", "true")).To(Succeed())
				Expect(command.Flags().Set("shoot-name", "name")).To(Succeed())
				Expect(command.Flags().Set("shoot-namespace", "namespace")).To(Succeed())
			})

			It("should return an error because the cluster is not a garden", func() {
				Expect(command.RunE(command, []string{token})).To(MatchError(ContainSubstring("`--print-connect-command` can only be used when the kubeconfig points to the garden cluster")))
			})

			It("should successfully print the connect command", func() {
				restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gardencorev1beta1.SchemeGroupVersion})
				restMapper.Add(gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"), meta.RESTScopeNamespace)

				fakeClient = fakeclient.NewClientBuilder().WithRESTMapper(restMapper).Build()
				clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).WithRESTConfig(restConfig).Build()

				Expect(command.RunE(command, []string{token})).To(Succeed())
				Eventually(stdOut).Should(Say(`gardenadm connect --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" some-host
`))
			})
		})
	})
})
