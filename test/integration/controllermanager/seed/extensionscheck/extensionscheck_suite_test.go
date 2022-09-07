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

package extensionscheck_test

import (
	"context"
	"testing"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func TestSeedExtensionsCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seed ExtensionsCheck Controller Integration Test Suite")
}

const testID = "extensionscheck-controller-test"

var (
	testRunID = testID + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

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
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: map[client.Object]cache.ObjectSelector{
				&gardencorev1beta1.ControllerInstallation{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
				&gardencorev1beta1.Seed{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		}),
	})
	Expect(err).NotTo(HaveOccurred())

	fakeClock = testclock.NewFakeClock(time.Now())
	//This is required so that the ExtensionsReady condition is created with appropriate lastUpdateTimestamp and lastTransitionTimestamp.
	DeferCleanup(test.WithVars(
		&gardencorev1beta1helper.Now, func() metav1.Time { return metav1.Time{Time: fakeClock.Now()} },
	))

	By("registering controller")
	Expect(addSeedExtensionsCheckControllerToManager(mgr)).To(Succeed())

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

func addSeedExtensionsCheckControllerToManager(mgr manager.Manager) error {
	c, err := controller.New(
		"seed-extension-check",
		mgr,
		controller.Options{
			Reconciler: seed.NewExtensionsCheckReconciler(
				testClient,
				config.SeedExtensionsCheckControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: syncPeriod},
					ConditionThresholds: []config.ConditionThreshold{{
						Type:     string(gardencorev1beta1.SeedExtensionsReady),
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
		&source.Kind{Type: &gardencorev1beta1.ControllerInstallation{}},
		mapper.EnqueueRequestsFrom(
			mapper.MapFunc(mapControllerInstallationToSeed),
			mapper.UpdateWithOldAndNew,
			log,
		),
	)
}

func mapControllerInstallationToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	controllerInstallation := obj.(*gardencorev1beta1.ControllerInstallation)
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name: controllerInstallation.Spec.SeedRef.Name,
		},
	}}
}
