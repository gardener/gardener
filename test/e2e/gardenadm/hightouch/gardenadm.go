// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hightouch

import (
	"context"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/test/e2e/gardenadm/common"
)

var _ = Describe("gardenadm high-touch scenario tests", Label("gardenadm", "high-touch"), func() {
	BeforeEach(OncePerOrdered, func(ctx SpecContext) {
		testRunID := utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		By("Ensuring fresh machine pods for test execution")
		statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: statefulSetName, Namespace: namespace}}
		Expect(RuntimeClient.Client().Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(Succeed())

		patch := client.MergeFrom(statefulSet.DeepCopy())
		metav1.SetMetaDataAnnotation(&statefulSet.Spec.Template.ObjectMeta, "test-run-id", testRunID)
		Expect(RuntimeClient.Client().Patch(ctx, statefulSet, patch)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(RuntimeClient.Client().Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(Succeed())
			progressing, _ := health.IsStatefulSetProgressing(statefulSet)
			g.Expect(progressing).To(BeFalse())
			g.Expect(health.CheckStatefulSet(statefulSet)).To(Succeed())
		}).Should(Succeed())
	}, NodeTimeout(time.Minute))

	Describe("Single-node control plane", Ordered, Label("single"), func() {
		It("should initialize as control plane node", func(ctx SpecContext) {
			stdOut, _ := execute(ctx, 0,
				"gardenadm", "init",
			)

			Eventually(ctx, stdOut).Should(gbytes.Say("not implemented"))
		}, SpecTimeout(time.Minute))

		It("should join as worker node", func(ctx SpecContext) {
			stdOut, _ := execute(ctx, 1,
				"gardenadm", "join",
			)

			Eventually(ctx, stdOut).Should(gbytes.Say("not implemented either"))
		}, SpecTimeout(time.Minute))
	})
})

// nolint:unparam
func execute(ctx context.Context, ordinal int, command ...string) (*gbytes.Buffer, *gbytes.Buffer) {
	GinkgoHelper()
	var stdOutBuffer, stdErrBuffer = gbytes.NewBuffer(), gbytes.NewBuffer()

	Expect(RuntimeClient.PodExecutor().ExecuteWithStreams(
		ctx,
		namespace,
		machinePodName(ordinal),
		ContainerName,
		nil,
		io.MultiWriter(stdOutBuffer, gexec.NewPrefixedWriter("[out] ", GinkgoWriter)),
		io.MultiWriter(stdErrBuffer, gexec.NewPrefixedWriter("[err] ", GinkgoWriter)),
		command...,
	)).To(Succeed())

	return stdOutBuffer, stdErrBuffer
}
