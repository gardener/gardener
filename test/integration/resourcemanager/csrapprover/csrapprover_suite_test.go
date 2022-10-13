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

package csrapprover_test

import (
	"context"
	"path/filepath"
	"testing"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	userpkg "k8s.io/apiserver/pkg/authentication/user"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/logger"
	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/csrapprover"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestKubeletCSRApproverController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kubelet Server CertificateSigningRequest Approver Controller Integration Test Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "kubelet-csr-autoapprove-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	scheme     *runtime.Scheme
	testEnv    *envtest.Environment
	testClient client.Client
	mgrClient  client.Client

	testNamespace *corev1.Namespace
	testRunID     string

	nodeName string
	userName string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{filepath.Join("testdata", "crd-machines.yaml")},
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

	By("creating test clients")
	scheme = runtime.NewScheme()
	Expect(kubernetesscheme.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(machinev1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:16]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	nodeName = "node-" + testRunID
	userName = "system:node:" + nodeName

	// We have to "fake" that our test client is the kubelet user because the .spec.username field in CSRs will also be
	// overwritten by the kube-apiserver to the user who created it. This would always fail the constraints of this
	// controller.
	user, err := testEnv.AddUser(
		envtest.User{Name: userName, Groups: []string{userpkg.SystemPrivilegedGroup}},
		&rest.Config{QPS: 1000.0, Burst: 2000.0},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(user).NotTo(BeNil())

	testClient, err = client.New(user.Config(), client.Options{Scheme: scheme})
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
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Namespace:          testNamespace.Name,
		NewCache: cache.BuilderWithOptions(cache.Options{
			DefaultSelector: cache.ObjectSelector{
				Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
			},
		}),
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("registering controller")
	targetClusterOpts := &resourcemanagercmd.TargetClusterOptions{Namespace: testNamespace.Name, RESTConfig: restConfig}
	Expect(targetClusterOpts.Complete()).To(Succeed())
	Expect(mgr.Add(targetClusterOpts.Completed().Cluster)).To(Succeed())

	Expect(csrapprover.AddToManagerWithOptions(mgr, csrapprover.ControllerConfig{
		MaxConcurrentWorkers: 5,
		TargetCluster:        targetClusterOpts.Completed().Cluster,
		Namespace:            testNamespace.Name,
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
