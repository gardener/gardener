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

package certificates_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestWebhookCertificates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Webhook Certificates Integration Test Suite")
}

var (
	ctx       = context.Background()
	mgrCancel context.CancelFunc
	log       logr.Logger

	testEnv    *envtest.Environment
	restConfig *rest.Config
	testClient client.Client

	extensionNamespace *corev1.Namespace
	shootNamespace     *corev1.Namespace
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
				filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join("..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
			},
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
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.SeedScheme})
	Expect(err).NotTo(HaveOccurred())

	By("creating test namespaces")
	extensionNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "webhook-certs-tests"}}
	Expect(testClient.Create(ctx, extensionNamespace)).To(Or(Succeed(), BeAlreadyExistsError()))

	shootNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shoot--foo--bar",
			Labels: map[string]string{
				"shoot.gardener.cloud/provider": providerType,
				"gardener.cloud/role":           "shoot",
			},
		},
	}
	Expect(testClient.Create(ctx, shootNamespace)).To(Or(Succeed(), BeAlreadyExistsError()))

	DeferCleanup(func() {
		By("deleting extension namespace")
		Expect(testClient.Delete(ctx, extensionNamespace)).To(Or(Succeed(), BeNotFoundError()))

		By("deleting shoot namespace")
		Expect(testClient.Delete(ctx, shootNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})
})
