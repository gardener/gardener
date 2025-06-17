// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mediumtouch

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/gardener/gardener/test/e2e/gardenadm/common"
)

var binaryPath string

// PrepareBinary builds the gardenadm binary.
func PrepareBinary() {
	By("Building gardenadm binary")
	var err error
	binaryPath, err = gexec.Build("../../../cmd/gardenadm")
	Expect(err).NotTo(HaveOccurred())
	logf.Log.Info("Using binary", "path", binaryPath)

	DeferCleanup(gexec.CleanupBuildArtifacts)
}

// NewCommand creates a new exec.Cmd for gardenadm.
func NewCommand(args ...string) *exec.Cmd { // #nosec G204 -- Used for e2e tests only.
	cmd := exec.Command(binaryPath, append([]string{"--log-level=debug"}, args...)...)
	cmd.Env = append(cmd.Env,
		clientcmd.RecommendedConfigPathEnvVar+"=../../../example/gardener-local/kind/local/kubeconfig",
		imagevector.OverrideEnv+"=../../../example/gardenadm-local/.imagevector-overwrite.yaml",
	)
	return cmd
}

// RunCommand runs the given exec.Cmd and returns the gexec.Session.
func RunCommand(cmd *exec.Cmd) *gexec.Session {
	GinkgoHelper()

	session, err := gexec.Start(
		cmd,
		gexec.NewPrefixedWriter("[out] ", GinkgoWriter),
		gexec.NewPrefixedWriter("[err] ", GinkgoWriter),
	)
	Expect(err).NotTo(HaveOccurred())

	return session
}

// Wait waits for the given gexec.Session to finish and returns the session.
func Wait(ctx context.Context, session *gexec.Session) *gexec.Session {
	GinkgoHelper()

	Eventually(ctx, session).Should(gexec.Exit(0))
	return session
}

// Run runs gardenadm with the given arguments and returns the gexec.Session.
func Run(args ...string) *gexec.Session {
	return RunCommand(NewCommand(args...))
}

// RunAndWait runs gardenadm with the given arguments and waits for the session to finish.
func RunAndWait(ctx context.Context, args ...string) *gexec.Session {
	return Wait(ctx, Run(args...))
}

// RunInMachine runs gardenadm in the given machine (sorted lexicographically) with the given arguments and returns the
// gbytes.Buffers.
func RunInMachine(ctx context.Context, technicalID string, ordinal int, args ...string) (*gbytes.Buffer, *gbytes.Buffer, error) {
	var stdOutBuffer, stdErrBuffer = gbytes.NewBuffer(), gbytes.NewBuffer()
	podName := machinePodName(ctx, technicalID, ordinal)
	err := RuntimeClient.PodExecutor().ExecuteWithStreams(
		ctx,
		technicalID,
		podName,
		ContainerName,
		nil,
		io.MultiWriter(stdOutBuffer, gexec.NewPrefixedWriter(fmt.Sprintf("[%s][out] ", podName), GinkgoWriter)),
		io.MultiWriter(stdErrBuffer, gexec.NewPrefixedWriter(fmt.Sprintf("[%s][err] ", podName), GinkgoWriter)),
		append([]string{"/opt/bin/gardenadm"}, args...)...,
	)
	return stdOutBuffer, stdErrBuffer, err
}

func machinePodName(ctx context.Context, technicalID string, ordinal int) string {
	GinkgoHelper()

	podList := &corev1.PodList{}
	Expect(RuntimeClient.Client().List(ctx, podList, client.InNamespace(technicalID), client.MatchingLabels{"app": "machine"})).To(Succeed())

	Expect(ordinal).To(BeNumerically("<", len(podList.Items)))

	slices.SortFunc(podList.Items, func(a, b corev1.Pod) int {
		return strings.Compare(a.Name, b.Name)
	})

	return podList.Items[ordinal].Name
}
