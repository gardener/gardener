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
	"fmt"
	"path/filepath"
	"testing"
	"time"

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/cmd/utils"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/seedadmission"
	"github.com/gardener/gardener/pkg/seedadmission/webhooks/admission/extensioncrds"
	"github.com/gardener/gardener/pkg/utils/test"
	"github.com/gardener/gardener/test/framework"
)

func TestSeedAdmissionController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SeedAdmissionController Suite")
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
	server.Register(extensioncrds.WebhookPath, &webhook.Admission{Handler: extensioncrds.New(logger)})
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
					Path: pointer.StringPtr(extensioncrds.WebhookPath),
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
					Path: pointer.StringPtr(extensioncrds.WebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version},
		}},
	}
}

var _ = Describe("Integration Test", func() {
	Describe("Extension CRDs Webhook Handler", func() {
		var (
			c         client.Client
			namespace = "shoot--foo--bar"

			crdObjects = []client.Object{
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupbuckets.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupentries.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "containerruntimes.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "controlplanes.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "extensions.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "infrastructures.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "networks.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "operatingsystemconfigs.extensions.gardener.cloud"}},
				&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "workers.extensions.gardener.cloud"}},
			}
			extensionObjects = []client.Object{
				&extensionsv1alpha1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
				&extensionsv1alpha1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
				&extensionsv1alpha1.ContainerRuntime{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.ControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Network{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
				&extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			}
			podObject = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "foo",
						Image: "foo:latest",
					}},
				},
			}

			deletionUnprotectedLabels    = map[string]string{common.GardenerDeletionProtected: "false"}
			deletionConfirmedAnnotations = map[string]string{common.ConfirmationDeletion: "true"}
		)

		BeforeEach(func() {
			c, err = client.New(restConfig, client.Options{Scheme: kubernetes.SeedScheme})
			Expect(err).NotTo(HaveOccurred())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			if err := c.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("applying CRDs")
			applier, err := kubernetes.NewChartApplierForConfig(restConfig)
			Expect(err).NotTo(HaveOccurred())
			repoRoot := filepath.Join("..", "..")
			Expect(applier.Apply(ctx, filepath.Join(repoRoot, "charts", "seed-bootstrap", "charts", "extensions"), "extensions", "")).To(Succeed())

			Eventually(func() bool {
				for _, object := range extensionObjects {
					err := c.Get(ctx, client.ObjectKeyFromObject(object), object)
					if meta.IsNoMatchError(err) {
						return false
					}
				}
				return true
			}, 1*time.Second, 50*time.Millisecond).Should(BeTrue())
		})

		objectID := func(obj client.Object) string {
			return fmt.Sprintf("%T/%s", obj, client.ObjectKeyFromObject(obj))
		}

		testDeletionUnconfirmed := func(ctx context.Context, obj client.Object) {
			Eventually(func() string {
				err := c.Delete(ctx, obj)
				return string(apierrors.ReasonForError(err))
			}, 1*time.Second, 50*time.Millisecond).Should(ContainSubstring("annotation to delete"), objectID(obj))
		}

		testDeletionConfirmed := func(ctx context.Context, obj client.Object) {
			Eventually(func() error {
				return c.Delete(ctx, obj)
			}, 1*time.Second, 50*time.Millisecond).ShouldNot(HaveOccurred(), objectID(obj))
			Eventually(func() bool {
				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
				return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
			}, 1*time.Second, 50*time.Millisecond).Should(BeTrue(), objectID(obj))
		}

		Context("custom resource definitions", func() {
			It("should admit the deletion because CRD has no protection label", func() {
				for _, obj := range crdObjects {
					// patch out default gardener.cloud/deletion-protected=true label
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetLabels(nil)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})

			It("should admit the deletion because CRD's protection label is not true", func() {
				for _, obj := range crdObjects {
					// override default gardener.cloud/deletion-protected=true label
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetLabels(deletionUnprotectedLabels)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})

			It("should prevent the deletion because CRD's protection label is true but deletion is not confirmed", func() {
				// CRDs in seed-bootstrap chart should have gardener.cloud/deletion-protected=true label by default
				for _, obj := range crdObjects {
					testDeletionUnconfirmed(ctx, obj)
				}
			})

			It("should admit the deletion because CRD's protection label is true and deletion is confirmed", func() {
				// CRDs in seed-bootstrap chart should have gardener.cloud/deletion-protected=true label by default
				for _, obj := range crdObjects {
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetAnnotations(deletionConfirmedAnnotations)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})
		})

		Context("extension resources", func() {
			BeforeEach(func() {
				By("creating extension test objects")
				_, err := test.EnsureTestResources(ctx, c, "testdata")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should prevent the deletion because deletion is not confirmed", func() {
				for _, obj := range extensionObjects {
					testDeletionUnconfirmed(ctx, obj)
				}
			})

			It("should admit the deletion because deletion is confirmed", func() {
				for _, obj := range extensionObjects {
					_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
						obj.SetAnnotations(deletionConfirmedAnnotations)
						return nil
					})
					Expect(err).NotTo(HaveOccurred(), objectID(obj))
					testDeletionConfirmed(ctx, obj)
				}
			})
		})

		Context("other resources", func() {
			It("should not block deletion of other resources", func() {
				Expect(c.Create(ctx, podObject)).To(Succeed())
				testDeletionConfirmed(ctx, podObject)
			})
		})
	})
})

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
