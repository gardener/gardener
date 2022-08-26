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

package backupbucketscheck_test

import (
	"context"
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func TestSeedBackupBucketsCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seed BackupBucketsCheck Controller Integration Test Suite")
}

const testID = "backupbucketscheck-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	fakeClock *testclock.FakeClock
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,SeedValidator,ExtensionValidator"},
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

	By("creating test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	By("registering controller")
	fakeClock = &testclock.FakeClock{}
	Expect(addSeedBackupBucketsCheckControllerToManager(mgr)).To(Succeed())

	By("starting manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).NotTo(HaveOccurred())
	}()

	DeferCleanup(func() {
		By("stopping manager")
		mgrCancel()
	})
})

func addSeedBackupBucketsCheckControllerToManager(mgr manager.Manager) error {
	c, err := controller.New(
		"seed-backupbuckets-check",
		mgr,
		controller.Options{
			Reconciler: seed.NewBackupBucketsCheckReconciler(
				testClient,
				config.SeedBackupBucketsCheckControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: syncPeriod},
					ConditionThresholds: []config.ConditionThreshold{{
						Type:     string(gardencorev1beta1.SeedBackupBucketsReady),
						Duration: metav1.Duration{Duration: conditionThreshold},
					}},
				},
				fakeClock,
			),
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.BackupBucket{}},
		mapper.EnqueueRequestsFrom(
			mapper.MapFunc(mapBackupBucketToSeed),
			mapper.UpdateWithOldAndNew,
			log,
		),
	)
}

func mapBackupBucketToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	backupBucket := obj.(*gardencorev1beta1.BackupBucket)
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name: *backupBucket.Spec.SeedName,
		},
	}}
}
