// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/bootstrap"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/test"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Bootstrap", func() {
	var (
		ctx = context.Background()

		globalOpts *cmd.Options
		stdErr     *Buffer
		command    *cobra.Command

		fakeClient client.Client
		clientSet  kubernetes.Interface
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, _, stdErr = clitest.NewTestIOStreams()
		globalOpts.Log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(stdErr))
		command = NewCommand(globalOpts)

		fakeClient = fakeclient.NewClientBuilder().Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		DeferCleanup(test.WithVar(&NewClientSetFromFile, func(string) (kubernetes.Interface, error) { return clientSet, nil }))
	})

	Describe("#RunE", func() {
		BeforeEach(func() {
			Expect(command.Flags().Set("kubeconfig", "some-path-to-kubeconfig")).To(Succeed())
			Expect(command.Flags().Set("config-dir", "some-path-to-config-dir")).To(Succeed())
		})

		Describe("safety check", func() {
			It("should abort the execution if gardener-operator is deployed on the targeted cluster", func() {
				Expect(fakeClient.Create(ctx, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "gardener-operator"},
				})).To(Succeed())
				Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring(`deployment "garden/gardener-operator" exists on the targeted cluster`)))
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
