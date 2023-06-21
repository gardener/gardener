// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseedset_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	managedseedsetcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
)

func TestManagedSeedSet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ControllerManager ManagedSeedSet Suite")
}

const testID = "managedseedset-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	mgrClient  client.Reader
	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	gardenNamespace *corev1.Namespace
	testRunID       string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,SeedValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ManagedSeedShoot,ManagedSeed,ShootManagedSeed,ShootValidator",
			},
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

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create garden namespace")
	gardenNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "garden",
		},
	}
	Expect(testClient.Create(ctx, gardenNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "gardenNamespace", client.ObjectKeyFromObject(gardenNamespace))

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
		Cache: cache.Options{
			DefaultSelector: cache.ObjectSelector{
				Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Register controller")
	Expect((&managedseedsetcontroller.Reconciler{
		Config: config.ManagedSeedSetControllerConfiguration{
			ConcurrentSyncs: pointer.Int(5),
			SyncPeriod:      metav1.Duration{Duration: 500 * time.Millisecond},
		},
	}).AddToManager(ctx, mgr)).To(Succeed())

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
