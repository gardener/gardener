// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	druidcorecrds "github.com/gardener/etcd-druid/api/core/v1alpha1/crds"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/logger"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller/garden/care"
	"github.com/gardener/gardener/pkg/operator/features"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestGarden(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Operator Garden Care Suite")
}

const testID = "garden-care-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig    *rest.Config
	testEnv       *envtest.Environment
	testClient    client.Client
	testClientSet kubernetes.Interface
	mgrClient     client.Client

	testRunID     string
	testNamespace *corev1.Namespace
	gardenName    string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	var err error
	By("Fetch Etcd CRD")
	k8sVersion, err := gardenerenvtest.GetK8SVersion()
	Expect(err).NotTo(HaveOccurred())
	etcdCRDGetter, err := etcd.NewCRDGetter(k8sVersion)
	Expect(err).NotTo(HaveOccurred())
	etcdCRD, err := etcdCRDGetter.GetCRD(druidcorecrds.ResourceNameEtcd)
	Expect(err).NotTo(HaveOccurred())

	By("Start test environment")
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_gardens.yaml"),
				filepath.Join("..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
			},
			CRDs: []*apiextensionsv1.CustomResourceDefinition{etcdCRD},
		},
		ErrorIfCRDPathMissing: true,
	}

	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: operatorclient.RuntimeScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespaces")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
	gardenName = testNamespace.Name
	testRunID = testNamespace.Name

	istioSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-system",
		},
	}
	Expect(testClient.Create(ctx, istioSystemNamespace)).To(Succeed())
	log.Info("Created istio-system namespace for test")

	DeferCleanup(func() {
		By("Delete test Namespaces")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		Expect(testClient.Delete(ctx, istioSystemNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  operatorclient.RuntimeScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&operatorv1alpha1.Garden{}: {
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

	By("Create test clientset")
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	gardenClientMap := fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForGarden(&operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{Name: gardenName}}), testClientSet).Build()

	By("Register controller")
	Expect((&care.Reconciler{
		Config: operatorconfigv1alpha1.OperatorConfiguration{
			Controllers: operatorconfigv1alpha1.ControllerConfiguration{
				GardenCare: operatorconfigv1alpha1.GardenCareControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
				},
			},
		},
		GardenNamespace: testNamespace.Name,
	}).AddToManager(mgr, gardenClientMap)).To(Succeed())

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
