// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicy_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestNetworkPolicy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ResourceManager NetworkPolicy Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "networkpolicy-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client
	mgrClient  client.Client

	testRunID string

	ingressControllerNamespace   = "ingress-ctrl-ns"
	ingressControllerPodSelector = metav1.LabelSelector{MatchLabels: map[string]string{"app": "ingress-controller"}}
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test clients")
	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:16]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetesscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetesscheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Register controller")
	Expect((&networkpolicy.Reconciler{
		Config: config.NetworkPolicyControllerConfig{
			ConcurrentSyncs:    pointer.Int(5),
			NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{testID: testRunID}}},
			IngressControllerSelector: &config.IngressControllerSelector{
				Namespace:   ingressControllerNamespace,
				PodSelector: ingressControllerPodSelector,
			},
		},
	}).AddToManager(ctx, mgr, mgr)).To(Succeed())

	// We create and delete namespace in every test, so let's ensure they get finalized.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
})
