// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardenaccess "github.com/gardener/gardener/pkg/operator/controller/garden/access"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestAccess(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Operator Garden Access Suite")
}

const testID = "garden-access-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	testEnv   *envtest.Environment
	mgrClient client.Client

	testRunID     string
	testNamespace *corev1.Namespace
	testSecret    *corev1.Secret
	tokenFilePath string

	fs afero.Fs
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
	}

	restConfig, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	testClient, err := client.New(restConfig, client.Options{Scheme: operatorclient.RuntimeScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespaces")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}

	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	testRunID = testNamespace.Name
	testSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNamespace.Name,
			Namespace: testNamespace.Name,
			Labels: map[string]string{
				testID: testRunID,
			},
		},
	}

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  operatorclient.RuntimeScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})

	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	fs = afero.NewOsFs()
	tokenFilePath = testRunID + ".test"

	By("Register controller")
	DeferCleanup(test.WithVar(&gardenaccess.CreateTemporaryFile, func(fs afero.Fs, _, _ string) (afero.File, error) {
		return fs.Create(tokenFilePath)
	}))

	DeferCleanup(func() {
		Expect(fs.Remove(tokenFilePath)).To(Succeed())
	})

	Expect((&gardenaccess.Reconciler{
		FS:     fs,
		Config: &config.OperatorConfiguration{},
	}).AddToManager(mgr, testSecret.Name, testSecret.Name)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
})
