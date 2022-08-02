// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package rootcapublisher_test

import (
	"context"
	"os"
	"testing"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/rootcapublisher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// TODO: This test suite is disabled because of a known flake in the test.
// Enable the test suite again after fixing the flake.

func TestRootCAPublisher(t *testing.T) {
	t.Skipf("Temporarily skipping the test suite because of known flake...")
	RegisterFailHandler(Fail)
	RunSpecs(t, "Root CA Controller Integration Test Suite")
}

const testID = "rootcapublisher-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client

	caCert []byte
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &envtest.Environment{}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("creating test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: k8sscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	const rootCAPath = "testdata/ca.crt"
	caCert, err = os.ReadFile(rootCAPath)
	Expect(err).To(BeNil())
	Expect(caCert).ToNot(BeEmpty())

	By("setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             k8sscheme.Scheme,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	By("registering controller")
	Expect(rootcapublisher.AddToManagerWithOptions(mgr, rootcapublisher.ControllerConfig{
		MaxConcurrentWorkers: 1,
		RootCAPath:           rootCAPath,
		TargetCluster:        mgr,
	})).To(Succeed())

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
