// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

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

		fakeClient = fakeclient.NewClientBuilder().Build()
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

		When("the join command should be printed", func() {
			BeforeEach(func() {
				Expect(command.Flags().Set("print-join-command", "true")).To(Succeed())
				Expect(command.Flags().Set("worker-pool-name", "test-pool")).To(Succeed())
			})

			It("should fail because there are no gardener-node-agent-secrets", func() {
				Expect(command.RunE(command, []string{token})).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
			})

			It("should successfully print the join command", func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-node-agent-test-pool",
					Namespace: "kube-system",
					Labels: map[string]string{
						"gardener.cloud/role":        "operating-system-config",
						"worker.gardener.cloud/pool": "test-pool",
					},
				}})).To(Succeed())

				Expect(command.RunE(command, []string{token})).To(Succeed())
				Eventually(stdOut).Should(Say(`gardenadm join --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" --gardener-node-agent-secret-name gardener-node-agent-test-pool some-host
`))
			})
		})

		When("the connect command should be printed", func() {
			BeforeEach(func() {
				Expect(command.Flags().Set("print-connect-command", "true")).To(Succeed())
				Expect(command.Flags().Set("shoot-name", "name")).To(Succeed())
				Expect(command.Flags().Set("shoot-namespace", "namespace")).To(Succeed())
			})

			It("should successfully print the connect command", func() {
				Expect(command.RunE(command, []string{token})).To(Succeed())
				Eventually(stdOut).Should(Say(`gardenadm connect --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" some-host
`))
			})
		})
	})
})
