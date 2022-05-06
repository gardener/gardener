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

package reconciler_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/managedresource"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"
	managerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

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
)

func TestManagedResourceController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManagedResource Controller Integration Test Suite")
}

const namespaceName = "test-namespace"

var (
	ctx       = context.Background()
	mgrCancel context.CancelFunc

	logger     logr.Logger
	testEnv    *envtest.Environment
	testClient client.Client

	filter *managerpredicate.ClassFilter

	namespace *corev1.Namespace
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(options *zap.Options) {
		options.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	logger = logf.Log.WithName("test")

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
	Expect(err).ToNot(HaveOccurred())
	Expect(restConfig).ToNot(BeNil())

	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.SeedScheme})
	Expect(err).ToNot(HaveOccurred())

	By("creating test namespace")
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	Expect(testClient.Create(ctx, namespace)).To(Or(Succeed(), BeAlreadyExistsError()))

	By("setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		MetricsBindAddress: "0",
		Scheme:             kubernetes.SeedScheme,
	})
	Expect(err).NotTo(HaveOccurred())

	By("registering controller")
	filter = predicate.NewClassFilter(managerpredicate.DefaultClass)
	Expect(managedresource.AddToManagerWithOptions(mgr, managedresource.ControllerConfig{
		MaxConcurrentWorkers: 5,
		SyncPeriod:           500 * time.Millisecond, // gotta go fast during tests

		TargetCluster: mgr,
		ClassFilter:   filter,
	})).To(Succeed())

	By("starting manager")
	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	By("deleting namespace")
	if testClient != nil {
		Expect(testClient.Delete(ctx, namespace)).To(Succeed())
	}

	By("stopping manager")
	if mgrCancel != nil {
		mgrCancel()
	}

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())
})
