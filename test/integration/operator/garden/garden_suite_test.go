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

package garden_test

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
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller/garden"
	"github.com/gardener/gardener/pkg/operator/features"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestGarden(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Controller Integration Test Suite")
}

const testID = "garden-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client
	mgrClient  client.Client

	testRunID     string
	testNamespace *corev1.Namespace
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	By("starting test environment")
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_gardens.yaml")},
		},
		ErrorIfCRDPathMissing: true,
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
	testClient, err = client.New(restConfig, client.Options{Scheme: operatorclient.RuntimeScheme})
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

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             operatorclient.RuntimeScheme,
		MetricsBindAddress: "0",
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: map[client.Object]cache.ObjectSelector{
				&operatorv1alpha1.Garden{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		}),
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("registering controller")
	chartsPath := filepath.Join("..", "..", "..", "..", charts.Path)
	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(chartsPath, "images.yaml"))
	Expect(err).NotTo(HaveOccurred())

	Expect((&garden.Reconciler{
		Config: config.OperatorConfiguration{
			Controllers: config.ControllerConfiguration{
				Garden: config.GardenControllerConfig{
					ConcurrentSyncs: pointer.Int(5),
					SyncPeriod:      &metav1.Duration{Duration: time.Minute},
				},
			},
		},
		ImageVector:     imageVector,
		GardenNamespace: testNamespace.Name,
	}).AddToManager(mgr)).To(Succeed())

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
