// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package etcdmember_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	druidcorecrds "github.com/gardener/etcd-druid/api/core/v1alpha1/crds"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/etcdmember"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestEtcdMember(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Shoot EtcdMember Suite")
}

const testID = "shoot-etcdmember-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	testRunID  string

	logBuffer bytes.Buffer
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(io.MultiWriter(GinkgoWriter, &logBuffer))))
	log = logf.Log.WithName(testID)

	k8sVersion, err := gardenerenvtest.GetK8SVersion()
	Expect(err).NotTo(HaveOccurred())

	By("Fetch Etcd CRD")
	etcdCRDs, err := druidcorecrds.GetAll(k8sVersion.String())
	Expect(err).NotTo(HaveOccurred())
	etcdCRDYAML, ok := etcdCRDs[druidcorecrds.ResourceNameEtcd]
	Expect(ok).To(BeTrue())
	etcdCRD, err := kubernetesutils.DecodeCRD(etcdCRDYAML)
	Expect(err).NotTo(HaveOccurred())

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-machine.sapcloud.io_machines.yaml"),
				},
				CRDs: []*apiextensionsv1.CustomResourceDefinition{etcdCRD},
			},
			ErrorIfCRDPathMissing: true,
		},
	}
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	testSchemeBuilder := runtime.NewSchemeBuilder(kubernetes.AddSeedSchemeToScheme)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  testScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Controller: controllerconfig.Controller{
			SkipNameValidation: new(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Register controller")
	Expect((&etcdmember.Reconciler{}).AddToManager(mgr)).To(Succeed())

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

func newNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Labels:       map[string]string{testID: testRunID},
		},
	}
}
