// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Compute", func() {
	Describe("#CreateBootstrapToken", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client
			clientSet  kubernetes.Interface

			tokenID     = "abcdef"
			tokenSecret = "1234567890abcdef"
			restConfig  = &rest.Config{Host: "some-host", TLSClientConfig: rest.TLSClientConfig{CAData: []byte("ca-data")}}

			stdOut     *Buffer
			globalOpts *cmd.Options
			opts       *Options
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().Build()
			clientSet = fakekubernetes.NewClientSetBuilder().
				WithClient(fakeClient).
				WithRESTConfig(restConfig).
				Build()

			globalOpts = &cmd.Options{}
			globalOpts.IOStreams, _, stdOut, _ = clitest.NewTestIOStreams()

			opts = &Options{
				Options:     globalOpts,
				Validity:    time.Hour,
				Description: "test",
			}
		})

		It("should return an error because a bootstrap token with the ID already exists", func() {
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-token-" + tokenID, Namespace: "kube-system"}})).To(Succeed())

			Expect(CreateBootstrapToken(ctx, clientSet, opts, tokenID, tokenSecret)).To(MatchError(ContainSubstring("already exists")))
		})

		It("should successfully create the secret and print the token", func() {
			Expect(CreateBootstrapToken(ctx, clientSet, opts, tokenID, tokenSecret)).To(Succeed())
			Eventually(stdOut).Should(Say(tokenID + "." + tokenSecret))
		})

		When("the join command should be printed", func() {
			BeforeEach(func() {
				opts.PrintJoinCommand = true
				opts.WorkerPoolName = "test-pool"
			})

			It("should fail because there are no gardener-node-agent-secrets", func() {
				Expect(CreateBootstrapToken(ctx, clientSet, opts, tokenID, tokenSecret)).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
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

				Expect(CreateBootstrapToken(ctx, clientSet, opts, tokenID, tokenSecret)).To(Succeed())
				Eventually(stdOut).Should(Say(`gardenadm join --bootstrap-token abcdef.1234567890abcdef --ca-certificate "Y2EtZGF0YQ==" --gardener-node-agent-secret-name gardener-node-agent-test-pool some-host`))
			})
		})
	})
})
