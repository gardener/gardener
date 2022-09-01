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

package conditions_test

import (
	"context"
	"testing"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	shootcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func TestShootConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoot Conditions Controller Integration Test Suite")
}

const testID = "conditions-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	testNamespace *corev1.Namespace
	testRunID     string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootQuotaValidator,ShootValidator,ShootTolerationRestriction,ManagedSeedShoot,ManagedSeed,ShootManagedSeed,ShootDNS"},
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

	By("creating test namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "garden",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
	testRunID = testNamespace.Name

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
		Namespace:          testNamespace.Name,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: map[client.Object]cache.ObjectSelector{
				&gardencorev1beta1.Shoot{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		}),
	})
	Expect(err).NotTo(HaveOccurred())

	By("setting up field indexes")
	Expect(indexer.AddManagedSeedShootName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	By("registering controller")
	Expect(addShootConditionsControllerToManager(mgr)).To(Succeed())

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

func addShootConditionsControllerToManager(mgr manager.Manager) error {
	c, err := controller.New(
		"shoot-conditions-controller",
		mgr,
		controller.Options{Reconciler: shootcontroller.NewShootConditionsReconciler(testClient)},
	)
	if err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &gardencorev1beta1.Shoot{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.Seed{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(mapSeedsToShoots), mapper.UpdateWithNew, c.GetLogger()),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return shootcontroller.FilterSeedForShootConditions(e.ObjectNew, e.ObjectOld, nil, false)
			},
		},
	)
}

func mapSeedsToShoots(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := reader.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, seed.Name), managedSeed); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get ManagedSeed for Seed", "seed", client.ObjectKeyFromObject(seed))
		}
		return nil
	}

	if managedSeed.Spec.Shoot == nil {
		return nil
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := reader.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get Shoot for ManagedSeed", "managedSeed", client.ObjectKeyFromObject(managedSeed))
		}
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: shoot.Namespace, Name: shoot.Name}}}
}
