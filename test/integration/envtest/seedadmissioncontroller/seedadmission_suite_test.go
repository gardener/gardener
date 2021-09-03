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

package seedadmissioncontroller_test

import (
	"context"
	"testing"

	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensioncrds"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/podschedulername"
	"github.com/gardener/gardener/test/framework"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestSeedAdmissionController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SeedAdmissionController Integration Test Suite")
}

var (
	ctx        context.Context
	ctxCancel  context.CancelFunc
	err        error
	logger     logr.Logger
	testEnv    *envtest.Environment
	restConfig *rest.Config
)

var _ = BeforeSuite(func() {
	utils.DeduplicateWarnings()
	ctx, ctxCancel = context.WithCancel(context.Background())

	logger = logzap.New(logzap.UseDevMode(true), logzap.WriteTo(GinkgoWriter), logzap.Level(zapcore.Level(1)))
	// enable manager and envtest logs
	logf.SetLogger(logger)

	By("starting test environment")
	testEnv = &envtest.Environment{
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			ValidatingWebhooks: []client.Object{getValidatingWebhookConfig()},
			MutatingWebhooks:   []client.Object{getMutatingWebhookConfig()},
		},
	}
	restConfig, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(restConfig).ToNot(BeNil())

	By("setting up manager")
	// setup manager in order to leverage dependency injection
	mgr, err := manager.New(restConfig, manager.Options{
		Port:    testEnv.WebhookInstallOptions.LocalServingPort,
		Host:    testEnv.WebhookInstallOptions.LocalServingHost,
		CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,

		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	By("setting up webhook server")
	server := mgr.GetWebhookServer()
	server.Register(extensioncrds.WebhookPath, &webhook.Admission{Handler: extensioncrds.New(logger)})
	server.Register(podschedulername.WebhookPath, &webhook.Admission{Handler: admission.HandlerFunc(podschedulername.DefaultShootControlPlanePodsSchedulerName)})
	server.Register(extensionresources.WebhookPath, &webhook.Admission{Handler: extensionresources.New(logger, true)})

	go func() {
		Expect(server.Start(ctx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	By("running cleanup actions")
	framework.RunCleanupActions()

	By("stopping manager")
	ctxCancel()

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

func getValidatingWebhookConfig() *admissionregistrationv1.ValidatingWebhookConfiguration {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-name",
			Namespace: "service-ns",
		},
	}

	webhookConfig := seedadmissioncontroller.GetValidatingWebhookConfig(nil, service)
	// envtest doesn't default the webhook config's GVK, so set it explicitly
	webhookConfig.SetGroupVersionKind(admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration"))

	return webhookConfig
}

func getMutatingWebhookConfig() *admissionregistrationv1.MutatingWebhookConfiguration {
	clientConfig := admissionregistrationv1.WebhookClientConfig{
		Service: &admissionregistrationv1.ServiceReference{
			Path: pointer.String(podschedulername.WebhookPath),
		},
	}

	webhookConfig := gardenerkubescheduler.GetMutatingWebhookConfig(clientConfig)
	// envtest doesn't default the webhook config's GVK, so set it explicitly
	webhookConfig.SetGroupVersionKind(admissionregistrationv1.SchemeGroupVersion.WithKind("MutatingWebhookConfiguration"))

	return webhookConfig
}
