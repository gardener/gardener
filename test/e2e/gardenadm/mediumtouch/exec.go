// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mediumtouch

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var binaryPath string

// PrepareBinary builds the gardenadm binary.
func PrepareBinary() {
	By("Building gardenadm binary")
	var err error
	binaryPath, err = gexec.Build("../../../cmd/gardenadm")
	Expect(err).NotTo(HaveOccurred())
	logf.Log.Info("Using binary", "path", binaryPath)

	DeferCleanup(func() {
		gexec.CleanupBuildArtifacts()
	})
}

// NewCommand creates a new exec.Cmd for gardenadm.
func NewCommand(args ...string) *exec.Cmd { // #nosec G204 -- Used for e2e tests only.
	return exec.Command(binaryPath, args...)
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
func Wait(session *gexec.Session) *gexec.Session {
	GinkgoHelper()

	Eventually(session).Should(gexec.Exit(0))
	return session
}

// Run runs gardenadm with the given arguments and returns the gexec.Session.
func Run(args ...string) *gexec.Session {
	return RunCommand(NewCommand(args...))
}

// RunAndWait runs gardenadm with the given arguments and waits for the session to finish.
func RunAndWait(args ...string) *gexec.Session {
	return Wait(Run(args...))
}
