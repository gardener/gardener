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

package envtest_test

import (
	"context"
	"testing"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestEnvTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardener Envtest Integration Test Suite")
}

var (
	ctx        = context.Background()
	err        error
	testEnv    *envtest.GardenerTestEnvironment
	restConfig *rest.Config

	testClient client.Client
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)).WithName("test"))

	By("starting test environment")
	testEnv = &envtest.GardenerTestEnvironment{}
	restConfig, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("running cleanup actions")
	framework.RunCleanupActions()

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())
})
