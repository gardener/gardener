// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seed_test

import (
	"context"
	_ "embed"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/api/indexer"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/seed"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestSeed(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seed Controller Integration Test Suite")
}

const testID = "seed-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig    *rest.Config
	testEnv       *gardenerenvtest.GardenerTestEnvironment
	testClient    client.Client
	testClientSet kubernetes.Interface
	mgrClient     client.Client

	testRunID     string
	testNamespace *corev1.Namespace
	seedName      string
	identity      = &gardencorev1beta1.Gardener{Version: "1.2.3"}

	//go:embed testdata/crd-managedresources.yaml
	managedResourcesCRD string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{filepath.Join("..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_gardens.yaml")},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator"},
		},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	scheme := kubernetes.SeedScheme
	Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

	By("creating test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	By("creating test namespace for test")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
	testRunID = testNamespace.Name
	seedName = "seed-" + testRunID

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: map[client.Object]cache.ObjectSelector{
				&gardencorev1beta1.Seed{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		}),
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	// We create the seed namespace in the garden and delete it after every test, so let's ensure it gets finalized.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

	By("setting up field indexes")
	Expect(indexer.AddBackupBucketSeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationSeedRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddShootSeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	By("creating test clientset")
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	By("registering controller")
	chartsPath := filepath.Join("..", "..", "..", "..", "..", charts.Path)
	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(chartsPath, "images.yaml"))
	Expect(err).NotTo(HaveOccurred())

	Expect((&seed.Reconciler{
		SeedClientSet: testClientSet,
		Config: config.GardenletConfiguration{
			Controllers: &config.GardenletControllerConfiguration{
				Seed: &config.SeedControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
				},
			},
			SNI: &config.SNI{
				Ingress: &config.SNIIngress{
					Namespace: pointer.String(testNamespace.Name + "-istio"),
				},
			},
			ETCDConfig: &config.ETCDConfig{
				BackupCompactionController: &config.BackupCompactionController{
					EnableBackupCompaction: pointer.Bool(false),
					EventsThreshold:        pointer.Int64(1),
					Workers:                pointer.Int64(1),
				},
				CustodianController: &config.CustodianController{
					Workers: pointer.Int64(1),
				},
				ETCDController: &config.ETCDController{
					Workers: pointer.Int64(1),
				},
			},
			SeedConfig: &config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: seedName,
					},
				},
			},
		},
		Identity:        identity,
		ImageVector:     imageVector,
		GardenNamespace: testNamespace.Name,
		ChartsPath:      chartsPath,
	}).AddToManager(mgr, mgr)).To(Succeed())

	By("starting manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("stopping manager")
		mgrCancel()
	})
})
