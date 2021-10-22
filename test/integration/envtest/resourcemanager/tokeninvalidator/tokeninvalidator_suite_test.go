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

package tokeninvalidator_test

import (
	"context"
	"testing"

	tokeninvalidatorcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/tokeninvalidator"
	tokeninvalidatorwebhook "github.com/gardener/gardener/pkg/resourcemanager/webhook/tokeninvalidator"
	"github.com/gardener/gardener/test/framework"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestTokenInvalidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TokenInvalidator Integration Test Suite")
}

var (
	ctx       = context.Background()
	mgrCancel context.CancelFunc

	logger     logr.Logger
	testEnv    *envtest.Environment
	restConfig *rest.Config
	testClient client.Client

	err error
)

var _ = BeforeSuite(func() {
	logger = logzap.New(logzap.UseDevMode(true), logzap.WriteTo(GinkgoWriter), logzap.Level(zapcore.Level(1)))
	logf.SetLogger(logger)

	By("starting test environment")
	testEnv = &envtest.Environment{
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			MutatingWebhooks: getMutatingWebhookConfigurations(),
		},
	}
	restConfig, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(restConfig).ToNot(BeNil())

	testClient, err = client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())

	By("setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Port:               testEnv.WebhookInstallOptions.LocalServingPort,
		Host:               testEnv.WebhookInstallOptions.LocalServingHost,
		CertDir:            testEnv.WebhookInstallOptions.LocalServingCertDir,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	By("registering controllers and webhooks")
	tokenInvalidatorControllerOpts := &tokeninvalidatorcontroller.ControllerOptions{}
	Expect(tokenInvalidatorControllerOpts.Complete()).To(Succeed())
	tokenInvalidatorControllerOpts.Completed().MaxConcurrentWorkers = 1
	tokenInvalidatorControllerOpts.Completed().TargetCluster = mgr

	Expect(tokeninvalidatorcontroller.AddToManager(mgr)).To(Succeed())
	Expect(tokeninvalidatorwebhook.AddToManager(mgr)).To(Succeed())

	By("starting manager")
	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)
	go func() {
		Expect(mgr.Start(mgrContext)).To(Succeed())
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

func getMutatingWebhookConfigurations() []admissionregistrationv1.MutatingWebhookConfiguration {
	var (
		scope              = admissionregistrationv1.AllScopes
		sideEffects        = admissionregistrationv1.SideEffectClassNone
		failurePolicy      = admissionregistrationv1.Fail
		matchPolicy        = admissionregistrationv1.Exact
		reinvocationPolicy = admissionregistrationv1.NeverReinvocationPolicy
	)

	return []admissionregistrationv1.MutatingWebhookConfiguration{
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
				Kind:       "MutatingWebhookConfiguration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-resource-manager",
			},
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:                    "token-invalidator.resources.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Path: pointer.String(tokeninvalidatorwebhook.WebhookPath),
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets"},
						Scope:       &scope,
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}},
				SideEffects:        &sideEffects,
				FailurePolicy:      &failurePolicy,
				MatchPolicy:        &matchPolicy,
				ReinvocationPolicy: &reinvocationPolicy,
				TimeoutSeconds:     pointer.Int32(10),
			}},
		},
	}
}
