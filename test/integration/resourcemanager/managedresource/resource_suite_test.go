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

package managedresource_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/managedresource"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"
	managerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestManagedResourceController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManagedResource Controller Integration Test Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "resource-controller-test"

var (
	ctx       = context.Background()
	mgrCancel context.CancelFunc
	log       logr.Logger

	testEnv    *envtest.Environment
	testClient client.Client

	testNamespace *corev1.Namespace

	filter *managerpredicate.ClassFilter
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(options *zap.Options) {
		options.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	log = logf.Log.WithName("test")

	By("starting test environment")
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
				filepath.Join("..", "..", "..", "..", "example", "seed-crds", "10-crd-autoscaling.k8s.io_hvpas.yaml"),
			},
		},
		ErrorIfCRDPathMissing: true,
	}

	restConfig, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("creating test client")
	testScheme := runtime.NewScheme()
	Expect(resourcemanagercmd.AddToSourceScheme(testScheme)).To(Succeed())
	Expect(resourcemanagercmd.AddToTargetScheme(testScheme)).To(Succeed())

	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	By("creating test namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			GenerateName: testID + "-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("setting up manager")
	mgrScheme := runtime.NewScheme()
	Expect(resourcemanagercmd.AddToSourceScheme(mgrScheme)).To(Succeed())

	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             mgrScheme,
		MetricsBindAddress: "0",
		Namespace:          testNamespace.Name,
	})
	Expect(err).NotTo(HaveOccurred())

	targetClusterOpts := &resourcemanagercmd.TargetClusterOptions{
		Namespace:  testNamespace.Name,
		RESTConfig: restConfig,
	}
	Expect(targetClusterOpts.Complete()).To(Succeed())
	Expect(mgr.Add(targetClusterOpts.Completed().Cluster)).To(Succeed())

	By("registering controller")
	filter = predicate.NewClassFilter(managerpredicate.DefaultClass)
	Expect(managedresource.AddToManagerWithOptions(mgr, managedresource.ControllerConfig{
		MaxConcurrentWorkers: 5,

		// gotta go fast during tests
		SyncPeriod:                    500 * time.Millisecond,
		RequeueAfterOnDeletionPending: 50 * time.Millisecond,

		ClassFilter:   filter,
		TargetCluster: targetClusterOpts.Completed().Cluster,
	})).To(Succeed())

	By("starting manager")
	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("stopping manager")
		mgrCancel()
	})
})
