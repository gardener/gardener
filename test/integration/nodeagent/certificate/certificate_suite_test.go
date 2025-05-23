// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificate_test

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	userpkg "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
)

func TestCertificate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration NodeAgent Certificate Suite")
}

const testID = "certificate-test"

var (
	log logr.Logger

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client

	machineName   string
	nodeAgentUser *envtest.AuthenticatedUser
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	machineName = "foo-machine"
	nodeAgentUser, err = testEnv.AddUser(
		envtest.User{Name: "gardener.cloud:node-agent:machine:" + machineName, Groups: []string{userpkg.SystemPrivilegedGroup}},
		&rest.Config{QPS: 1000.0, Burst: 2000.0},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(nodeAgentUser).NotTo(BeNil())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{})
	Expect(err).NotTo(HaveOccurred())
})
