// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package list_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/list"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
	"github.com/gardener/gardener/pkg/utils/test"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("List", func() {
	var (
		ctx = context.Background()

		globalOpts *cmd.Options
		stdOut     *Buffer
		command    *cobra.Command

		fakeClient client.Client
		clientSet  kubernetes.Interface
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, stdOut, _ = clitest.NewTestIOStreams()
		command = NewCommand(globalOpts)

		fakeClient = fakeclient.
			NewClientBuilder().
			WithIndex(&corev1.Secret{}, "type", func(obj client.Object) []string { return []string{string(obj.(*corev1.Secret).Type)} }).
			Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		DeferCleanup(test.WithVar(&tokenutils.CreateClientSet, func(context.Context, logr.Logger) (kubernetes.Interface, error) { return clientSet, nil }))
		DeferCleanup(test.WithVar(&Now, func() metav1.Time { return metav1.NewTime(time.Date(2050, 5, 5, 5, 5, 5, 0, time.UTC)) }))
		DeferCleanup(test.WithVar(&bootstraptoken.Now, func() metav1.Time { return metav1.NewTime(time.Date(2050, 5, 5, 5, 5, 5, 0, time.UTC)) }))
	})

	Describe("#RunE", func() {
		It("should print that no resources were found", func() {
			Expect(command.RunE(command, nil)).To(Succeed())

			Eventually(stdOut).Should(Say("No resources found."))
		})

		When("there are bootstrap tokens", func() {
			BeforeEach(func() {
				_, err := bootstraptoken.ComputeBootstrapTokenWithSecret(ctx, fakeClient, "token1", "token1secret1234", "1", time.Hour)
				Expect(err).NotTo(HaveOccurred())
				_, err = bootstraptoken.ComputeBootstrapTokenWithSecret(ctx, fakeClient, "token2", "token2secret5678", "2", 2*time.Hour)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should print a nice table without sensitive information", func() {
				Expect(command.RunE(command, nil)).To(Succeed())

				Eventually(stdOut).Should(Say(`NAME\s+TOKEN ID\s+EXPIRATION\s+DESCRIPTION\s+AGE
bootstrap-token-token1\s+token1\s+ 60m\s+\(2050-05-05T06:05:05Z\)\s+1\s+<unknown>
bootstrap-token-token2\s+token2\s+120m\s+\(2050-05-05T07:05:05Z\)\s+2\s+<unknown>
`))
			})

			It("should print a nice table with sensitive information", func() {
				Expect(command.Flags().Set("with-token-secret", "true")).To(Succeed())
				Expect(command.RunE(command, nil)).To(Succeed())

				Eventually(stdOut).Should(Say(`NAME\s+TOKEN ID\s+TOKEN SECRET\s+TOKEN\s+EXPIRATION\s+DESCRIPTION\s+AGE
bootstrap-token-token1\s+token1\s+token1secret1234\s+token1.token1secret1234\s+60m\s+\(2050-05-05T06:05:05Z\)\s+1\s+<unknown>
bootstrap-token-token2\s+token2\s+token2secret5678\s+token2.token2secret5678\s+120m\s+\(2050-05-05T07:05:05Z\)\s+2\s+<unknown>
`))
			})

			It("should fail because it cannot parse the expiration time", func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-987654", Namespace: "kube-system"},
					Type:       corev1.SecretTypeBootstrapToken,
					Data:       map[string][]byte{"expiration": []byte("cannot-parse")},
				})).To(Succeed())

				Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("failed parsing the expiration time")))
			})
		})
	})
})
