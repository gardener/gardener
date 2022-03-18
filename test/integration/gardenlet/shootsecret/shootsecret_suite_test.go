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

package shootsecret_test

import (
	"context"
	"path/filepath"
	"testing"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	shootsecretcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/shootsecret"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func TestShootSecret(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ShootSecret Controller Integration Test Suite")
}

var (
	ctx       = context.Background()
	mgrCancel context.CancelFunc

	testEnv    *gardenerenvtest.GardenerTestEnvironment
	restConfig *rest.Config
	testClient client.Client
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logzap.New(logzap.UseDevMode(true), logzap.WriteTo(GinkgoWriter)))

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{filepath.Join("..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_clusters.yaml")},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=ResourceReferenceManager,ExtensionValidator,ShootQuotaValidator,ShootValidator,ShootTolerationRestriction"},
		},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	scheme := kubernetes.GardenScheme
	Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())

	testClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(addControllerToManager(mgr)).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	By("stopping manager")
	mgrCancel()

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

func addControllerToManager(mgr manager.Manager) error {
	c, err := controller.New("shootsecret-controller", mgr, controller.Options{
		Reconciler: shootsecretcontroller.NewReconciler(testClient, testClient, logger.AddWriter(logger.NewLogger("info", ""), GinkgoWriter)),
	})
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return shootsecretcontroller.LabelsPredicate(e.Object.GetLabels())
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return shootsecretcontroller.LabelsPredicate(e.ObjectNew.GetLabels())
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return shootsecretcontroller.LabelsPredicate(e.Object.GetLabels())
			},
		},
	)
}
