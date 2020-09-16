// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestBackupBucket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Controller BackupBucket Test Suite")
}

var (
	ctx        = context.Background()
	err        error
	logger     *logrus.Entry
	testEnv    *envtest.Environment
	restConfig *rest.Config
)

var _ = Describe("BackupBucket Controller", func() {
	BeforeSuite(func() {
		// enable manager logs
		logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

		log := logrus.New()
		log.SetOutput(GinkgoWriter)
		logger = logrus.NewEntry(log)

		By("starting test environment")
		repoRoot := filepath.Join("..", "..", "..", "..")
		testEnv = &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join(repoRoot, "charts", "seed-bootstrap", "templates", "extensions", "crd-cluster.yaml"),
					filepath.Join(repoRoot, "charts", "seed-bootstrap", "templates", "extensions", "crd-backupbucket.yaml"),
				},
			},
		}

		restConfig, err = testEnv.Start()
		Expect(err).ToNot(HaveOccurred())
		Expect(restConfig).ToNot(BeNil())
	})

	AfterSuite(func() {
		By("running cleanup actions")
		framework.RunCleanupActions()

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})
})
