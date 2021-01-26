// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedadmission_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/zap/zapcore"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
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

	"github.com/gardener/gardener/cmd/utils"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/seedadmission"
	"github.com/gardener/gardener/test/framework"
)

func TestSeedadmission(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seed Admission Suite")
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
	})
	Expect(err).NotTo(HaveOccurred())

	By("setting up webhook server")
	server := mgr.GetWebhookServer()
	server.Register(seedadmission.ExtensionDeletionProtectionWebhookPath, &webhook.Admission{Handler: &seedadmission.ExtensionDeletionProtection{}})
	server.Register(seedadmission.GardenerShootControlPlaneSchedulerWebhookPath, &webhook.Admission{Handler: admission.HandlerFunc(seedadmission.DefaultShootControlPlanePodsSchedulerName)})

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

func getValidatingWebhookConfig() *admissionregistrationv1beta1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1beta1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-seed-admission-controller",
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{
			Name: "crds.seed.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1beta1.RuleWithOperations{{
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   []string{apiextensionsv1.GroupName},
					APIVersions: []string{apiextensionsv1beta1.SchemeGroupVersion.Version, apiextensionsv1.SchemeGroupVersion.Version},
					Resources:   []string{"customresourcedefinitions"},
				},
				Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Delete},
			}},
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Path: pointer.StringPtr(seedadmission.ExtensionDeletionProtectionWebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version},
		}, {
			Name: "crs.seed.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1beta1.RuleWithOperations{{
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   []string{extensionsv1alpha1.SchemeGroupVersion.Group},
					APIVersions: []string{extensionsv1alpha1.SchemeGroupVersion.Version},
					Resources: []string{
						"backupbuckets",
						"backupentries",
						"containerruntimes",
						"controlplanes",
						"extensions",
						"infrastructures",
						"networks",
						"operatingsystemconfigs",
						"workers",
					},
				},
				Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Delete},
			}},
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Path: pointer.StringPtr(seedadmission.ExtensionDeletionProtectionWebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version},
		}},
	}
}

func getMutatingWebhookConfig() *admissionregistrationv1beta1.MutatingWebhookConfiguration {
	scope := admissionregistrationv1beta1.NamespacedScope

	return &admissionregistrationv1beta1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1beta1",
			Kind:       "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-seed-admission-controller",
		},
		Webhooks: []admissionregistrationv1beta1.MutatingWebhook{{
			Name: "kube-scheduler.scheduling.gardener.cloud",
			Rules: []admissionregistrationv1beta1.RuleWithOperations{{
				Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   []string{corev1.GroupName},
					APIVersions: []string{corev1.SchemeGroupVersion.Version},
					Scope:       &scope,
					Resources:   []string{"pods"},
				},
			}},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
				},
			},
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Path: pointer.StringPtr(seedadmission.GardenerShootControlPlaneSchedulerWebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version},
		}},
	}
}

func expectAllowed(response admission.Response, reason gomegatypes.GomegaMatcher, optionalDescription ...interface{}) {
	Expect(response.Allowed).To(BeTrue(), optionalDescription...)
	Expect(string(response.Result.Reason)).To(reason, optionalDescription...)
}

func expectPatched(response admission.Response, reason gomegatypes.GomegaMatcher, patches []jsonpatch.JsonPatchOperation, optionalDescription ...interface{}) {
	expectAllowed(response, reason, optionalDescription...)
	Expect(response.Patches).To(Equal(patches))
}

func expectDenied(response admission.Response, reason gomegatypes.GomegaMatcher, optionalDescription ...interface{}) {
	Expect(response.Allowed).To(BeFalse(), optionalDescription...)
	Expect(response.Result.Code).To(BeEquivalentTo(http.StatusForbidden), optionalDescription...)
	Expect(string(response.Result.Reason)).To(reason, optionalDescription...)
}

func expectErrored(response admission.Response, code, err gomegatypes.GomegaMatcher, optionalDescription ...interface{}) {
	Expect(response.Allowed).To(BeFalse(), optionalDescription...)
	Expect(response.Result.Code).To(code, optionalDescription...)
	Expect(response.Result.Message).To(err, optionalDescription...)
}
