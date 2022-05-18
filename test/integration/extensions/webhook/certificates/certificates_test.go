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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/certificates"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	servicePort = 12345

	providerName = "provider-test"
	providerType = "test"

	seedWebhookName, seedWebhookPath   = "seed-webhook", "seed-path"
	shootWebhookName, shootWebhookPath = "shoot-webhook", "shoot-path"
)

var _ = Describe("Certificates tests", func() {
	var (
		err       error
		mgr       manager.Manager
		codec     = newCodec(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		fakeClock *clock.FakeClock

		cluster            *extensionsv1alpha1.Cluster
		shootNetworkPolicy *networkingv1.NetworkPolicy

		seedWebhook              admissionregistrationv1.MutatingWebhook
		shootWebhook             admissionregistrationv1.MutatingWebhook
		seedWebhookConfig        *admissionregistrationv1.MutatingWebhookConfiguration
		shootWebhookConfig       *admissionregistrationv1.MutatingWebhookConfiguration
		atomicShootWebhookConfig *atomic.Value

		failurePolicyFail        = admissionregistrationv1.Fail
		matchPolicyExact         = admissionregistrationv1.Exact
		sideEffectsNone          = admissionregistrationv1.SideEffectClassNone
		reinvocationPolicy       = admissionregistrationv1.NeverReinvocationPolicy
		scope                    = admissionregistrationv1.AllScopes
		timeoutSeconds     int32 = 10
	)

	BeforeEach(func() {
		fakeClock = clock.NewFakeClock(time.Now())

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootNamespace.Name,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{Object: &gardencorev1beta1.CloudProfile{}},
				Seed:         runtime.RawExtension{Object: &gardencorev1beta1.Seed{}},
				Shoot:        runtime.RawExtension{Object: &gardencorev1beta1.Shoot{}},
			},
		}
		shootNetworkPolicy = shoot.GetNetworkPolicyMeta(shootNamespace.Name, providerName)

		seedWebhook = admissionregistrationv1.MutatingWebhook{
			Name: fmt.Sprintf("%s.%s.extensions.gardener.cloud", seedWebhookName, providerType),
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "gardener-extension-" + providerName,
					Namespace: extensionNamespace.Name,
					Path:      pointer.String("/" + seedWebhookPath),
					Port:      pointer.Int32(443),
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}, Scope: &scope},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				},
			},
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			FailurePolicy:           &failurePolicyFail,
			MatchPolicy:             &matchPolicyExact,
			SideEffects:             &sideEffectsNone,
			TimeoutSeconds:          &timeoutSeconds,
			ReinvocationPolicy:      &reinvocationPolicy,
			NamespaceSelector:       &metav1.LabelSelector{},
			ObjectSelector:          &metav1.LabelSelector{},
		}
		shootWebhook = admissionregistrationv1.MutatingWebhook{
			Name: fmt.Sprintf("%s.%s.extensions.gardener.cloud", shootWebhookName, providerType),
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: pointer.String("https://gardener-extension-" + providerName + "." + extensionNamespace.Name + ":443/" + shootWebhookPath),
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts"}, Scope: &scope},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				},
			},
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			FailurePolicy:           &failurePolicyFail,
			MatchPolicy:             &matchPolicyExact,
			SideEffects:             &sideEffectsNone,
			TimeoutSeconds:          &timeoutSeconds,
			ReinvocationPolicy:      &reinvocationPolicy,
			NamespaceSelector:       &metav1.LabelSelector{},
			ObjectSelector:          &metav1.LabelSelector{},
		}

		seedWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + providerName},
			Webhooks:   []admissionregistrationv1.MutatingWebhook{seedWebhook},
		}
		shootWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + providerName + "-shoot"},
			Webhooks:   []admissionregistrationv1.MutatingWebhook{shootWebhook},
		}
	})

	JustBeforeEach(func() {
		By("setting up manager")
		mgr, err = manager.New(restConfig, manager.Options{
			Scheme:             kubernetes.SeedScheme,
			MetricsBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())

		By("registering webhooks")
		var (
			serverOptions = &webhookcmd.ServerOptions{
				Mode:        extensionswebhook.ModeService,
				ServicePort: servicePort,
				Namespace:   extensionNamespace.Name,
			}
			switchOptions = webhookcmd.NewSwitchOptions(
				webhookcmd.Switch(seedWebhookName, newSeedWebhook),
				webhookcmd.Switch(shootWebhookName, newShootWebhook),
			)
			webhookOptions = webhookcmd.NewAddToManagerOptions(providerName, providerType, serverOptions, switchOptions)
		)

		Expect(webhookOptions.Complete()).To(Succeed())
		webhookConfig := webhookOptions.Completed()
		webhookConfig.Clock = fakeClock
		atomicShootWebhookConfig, err = webhookConfig.AddToManager(ctx, mgr)
		Expect(err).NotTo(HaveOccurred())

		By("verifying certificates exist on disk")
		serverCert, err := os.ReadFile(filepath.Join(mgr.GetWebhookServer().CertDir, "tls.crt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(serverCert).NotTo(BeEmpty())

		serverKey, err := os.ReadFile(filepath.Join(mgr.GetWebhookServer().CertDir, "tls.key"))
		Expect(err).NotTo(HaveOccurred())
		Expect(serverKey).NotTo(BeEmpty())

		By("starting manager")
		var mgrContext context.Context
		mgrContext, mgrCancel = context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("stopping manager")
			mgrCancel()
		})

		By("verifying CA bundle was written in atomic shoot webhook config")
		Eventually(func() []byte {
			return atomicShootWebhookConfig.Load().(*admissionregistrationv1.MutatingWebhookConfiguration).Webhooks[0].ClientConfig.CABundle
		}).ShouldNot(BeEmpty())
	})

	Context("seed webhook does not yet exist", func() {
		It("should create the webhook and inject the CA bundle", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
				g.Expect(extensionswebhook.InjectCABundleIntoWebhookConfig(seedWebhookConfig, nil)).To(Succeed())
				g.Expect(seedWebhookConfig.Webhooks).To(ConsistOf(seedWebhook))
			}).Should(Succeed())

			Eventually(func(g Gomega) []byte {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
				return seedWebhookConfig.Webhooks[0].ClientConfig.CABundle
			}).Should(Not(BeEmpty()))
		})
	})

	Context("certificate rotation", func() {
		BeforeEach(func() {
			By("preparing existing shoot webhook resources")
			Expect(testClient.Create(ctx, shootNetworkPolicy)).To(Succeed())
			Expect(testClient.Create(ctx, cluster)).To(Succeed())
			Expect(genericactuator.ReconcileShootWebhookConfig(ctx, testClient, shootNamespace.Name, providerName, servicePort, shootWebhookConfig, &extensions.Cluster{Shoot: &gardencorev1beta1.Shoot{}})).To(Succeed())

			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, shootNetworkPolicy)).To(Or(Succeed(), BeNotFoundError()))
				Expect(testClient.Delete(ctx, cluster)).To(Or(Succeed(), BeNotFoundError()))
			})

			DeferCleanup(test.WithVars(
				&certificates.DefaultSyncPeriod, 100*time.Millisecond,
				&secretutils.GenerateKey, secretutils.FakeGenerateKey,
			))
		})

		It("should rotate the certificates and update the webhook configs", func() {
			var caBundle1, caBundle2, serverCert1 []byte

			By("retrieving CA bundle (before first reconciliation)")
			Eventually(func(g Gomega) []byte {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
				caBundle1 = seedWebhookConfig.Webhooks[0].ClientConfig.CABundle
				return caBundle1
			}).Should(Not(BeEmpty()))

			Eventually(func(g Gomega) []byte {
				g.Expect(getShootWebhookConfig(codec, shootWebhookConfig)).To(Succeed())
				return shootWebhookConfig.Webhooks[0].ClientConfig.CABundle
			}).Should(Equal(caBundle1))

			By("reading generated server certificate from disk")
			Eventually(func(g Gomega) []byte {
				serverCert1, err = os.ReadFile(filepath.Join(mgr.GetWebhookServer().CertDir, "tls.crt"))
				g.Expect(err).NotTo(HaveOccurred())
				return serverCert1
			}).Should(Not(BeEmpty()))

			Eventually(func(g Gomega) []byte {
				serverKey1, err := os.ReadFile(filepath.Join(mgr.GetWebhookServer().CertDir, "tls.key"))
				g.Expect(err).NotTo(HaveOccurred())
				return serverKey1
			}).Should(Not(BeEmpty()))

			By("retrieving CA bundle again (after validity has expired)")
			fakeClock.Step(certificates.CACertificateValidity)

			Eventually(func(g Gomega) []byte {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
				caBundle2 = seedWebhookConfig.Webhooks[0].ClientConfig.CABundle
				return caBundle2
			}).Should(And(
				Not(BeEmpty()),
				Not(Equal(caBundle1)),
			))

			Eventually(func(g Gomega) []byte {
				g.Expect(getShootWebhookConfig(codec, shootWebhookConfig)).To(Succeed())
				return shootWebhookConfig.Webhooks[0].ClientConfig.CABundle
			}).Should(Equal(caBundle2))

			By("reading re-generated server certificate from disk")
			Eventually(func(g Gomega) []byte {
				serverCert2, err := os.ReadFile(filepath.Join(mgr.GetWebhookServer().CertDir, "tls.crt"))
				g.Expect(err).NotTo(HaveOccurred())
				return serverCert2
			}).Should(And(
				Not(BeEmpty()),
				Not(Equal(serverCert1)),
			))

			// we don't assert that the server key changed since we have overwritten the 'GenerateKey' function with
			// a fake implementation above (hence, it cannot change)
		})
	})

	Context("legacy secret", func() {
		var legacySecret *corev1.Secret

		BeforeEach(func() {
			legacySecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-webhook-cert", Namespace: extensionNamespace.Name}}
			Expect(testClient.Create(ctx, legacySecret)).To(Succeed())
		})

		It("should delete the legacy webhook certificate secret", func() {
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(legacySecret), &corev1.Secret{})
			}).Should(BeNotFoundError())
		})
	})
})

func newSeedWebhook(_ manager.Manager) (*extensionswebhook.Webhook, error) {
	return &extensionswebhook.Webhook{
		Name:     seedWebhookName,
		Path:     seedWebhookPath,
		Provider: providerType,
		Types:    []extensionswebhook.Type{{Obj: &corev1.Service{}}},
		Target:   extensionswebhook.TargetSeed,
		Handler:  http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}),
	}, nil
}

func newShootWebhook(_ manager.Manager) (*extensionswebhook.Webhook, error) {
	return &extensionswebhook.Webhook{
		Name:     shootWebhookName,
		Path:     shootWebhookPath,
		Provider: providerType,
		Types:    []extensionswebhook.Type{{Obj: &corev1.ServiceAccount{}}},
		Target:   extensionswebhook.TargetShoot,
		Handler:  http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}),
	}, nil
}

func newCodec(scheme *runtime.Scheme, codec serializer.CodecFactory, serializer *json.Serializer) runtime.Codec {
	var groupVersions []schema.GroupVersion
	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	return codec.CodecForVersions(serializer, serializer, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
}

func getShootWebhookConfig(codec runtime.Codec, shootWebhookConfig *admissionregistrationv1.MutatingWebhookConfiguration) error {
	managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extension-controlplane-shoot-webhooks", Namespace: shootNamespace.Name}}
	if err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret); err != nil {
		return err
	}

	_, _, err := codec.Decode(managedResourceSecret.Data["mutatingwebhookconfiguration____"+shootWebhookConfig.Name+".yaml"], nil, shootWebhookConfig)
	return err
}
