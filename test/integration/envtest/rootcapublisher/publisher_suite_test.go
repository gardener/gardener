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
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"

	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/gardener/gardener/pkg/resourcemanager/controller/rootcapublisher"
	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestRootCAPublisher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Root CA Controller Integration Test Suite")
}

var (
	ctx        = context.Background()
	restConfig *rest.Config
	testEnv    *envtest.Environment
	mgrCancel  context.CancelFunc

	testClient client.Client
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logzap.New(logzap.UseDevMode(true), logzap.WriteTo(GinkgoWriter)))

	By("starting test environment")
	var err error
	testEnv = &envtest.Environment{}
	restConfig, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(restConfig).ToNot(BeNil())

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             k8sscheme.Scheme,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	cl, err := cluster.New(restConfig, func(opts *cluster.Options) {
		opts.Scheme = k8sscheme.Scheme
		opts.NewClient = func(_ cache.Cache, config *rest.Config, opts client.Options, _ ...client.Object) (client.Client, error) {
			testClient, err = client.New(config, opts)
			if err != nil {
				return nil, err
			}

			return testClient, nil
		}
	})
	Expect(err).ToNot(HaveOccurred())

	err = rootcapublisher.AddToManagerWithOptions(mgr, rootcapublisher.ControllerConfig{
		MaxConcurrentWorkers: 1,
		RootCAPath:           "testdata/dummy.crt",
		TargetCluster:        cl,
	})
	Expect(err).ToNot(HaveOccurred())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		err := mgr.Start(mgrContext)
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("stopping manager")
	mgrCancel()

	By("running cleanup actions")
	framework.RunCleanupActions()

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())
})
