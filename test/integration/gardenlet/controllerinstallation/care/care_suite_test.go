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

package care_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func TestControllerInstallationCare(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerInstallationCare Controller Integration Test Suite")
}

const (
	testID     = "controllerinstallation-care-controller-test"
	syncPeriod = 100 * time.Millisecond
)

var (
	testRunID string

	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	gardenNamespace *corev1.Namespace
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{filepath.Join("..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml")},
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

	scheme := kubernetes.GardenScheme
	Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())

	By("creating testClient")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	By("creating garden namespace for test")
	gardenNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}

	Expect(testClient.Create(ctx, gardenNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", gardenNamespace.Name)
	testRunID = gardenNamespace.Name

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, gardenNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	DeferCleanup(test.WithVars(
		&controllerinstallation.ManagedResourcesNamespace, gardenNamespace.Name,
	))

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
		Namespace:          gardenNamespace.Name,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: map[client.Object]cache.ObjectSelector{
				&gardencorev1beta1.ControllerInstallation{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		}),
	})
	Expect(err).NotTo(HaveOccurred())

	By("registering controller")
	Expect(addControllerInstallationCareControllerToManager(mgr)).To(Succeed())

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

func addControllerInstallationCareControllerToManager(mgr manager.Manager) error {
	c, err := controller.New("controllerinstallation-care-controller", mgr, controller.Options{
		Reconciler: controllerinstallation.NewCareReconciler(testClient, testClient, config.ControllerInstallationCareControllerConfiguration{
			SyncPeriod: &metav1.Duration{Duration: syncPeriod},
		}),
	})
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.ControllerInstallation{}},
		&handler.EnqueueRequestForObject{},
	)
}
