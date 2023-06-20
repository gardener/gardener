// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package state_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/state"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestState(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Shoot State Suite")
}

const testID = "shoot-state-controller-test"

var (
	ctx       = context.Background()
	log       logr.Logger
	fakeClock *testclock.FakeClock

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	mgrClient  client.Client

	projectNamespace *corev1.Namespace
	seedNamespace    *corev1.Namespace
	testRunID        string
	seedName         string

	syncPeriod = 500 * time.Millisecond
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_backupentries.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_clusters.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_containerruntimes.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_controlplanes.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_dnsrecords.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_extensions.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_infrastructures.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_networks.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_operatingsystemconfigs.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_workers.yaml"),
				},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator"},
		},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	testSchemeBuilder := runtime.NewSchemeBuilder(
		kubernetes.AddGardenSchemeToScheme,
		extensionsv1alpha1.AddToScheme,
	)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	seedName = "seed-" + testRunID

	By("Create project Namespace")
	projectNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "garden-" + testRunID,
			Labels: map[string]string{
				testID: testRunID,
			},
		},
	}
	Expect(testClient.Create(ctx, projectNamespace)).To(Succeed())
	log.Info("Created Namespace for project", "namespaceName", projectNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, projectNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create Namespace for shoot in seed")
	seedNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shoot--namespace-" + testRunID,
			Labels: map[string]string{
				testID: testRunID,
			},
		},
	}
	Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())
	log.Info("Created Namespace for shoot in seed", "namespaceName", seedNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, seedNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             testScheme,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	fakeClock = testclock.NewFakeClock(time.Now().Round(time.Second))

	By("Register controller")
	Expect((&state.Reconciler{
		Config: config.ShootStateControllerConfiguration{
			ConcurrentSyncs: pointer.Int(5),
			SyncPeriod:      &metav1.Duration{Duration: syncPeriod},
		},
		Clock:    fakeClock,
		SeedName: seedName,
	}).AddToManager(mgr, mgr, mgr)).To(Succeed())

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
