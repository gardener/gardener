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

package seed_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed controller tests", func() {
	var (
		seed *gardencorev1beta1.Seed
	)

	JustBeforeEach(func() {
		DeferCleanup(test.WithVar(&secretutils.GenerateKey, secretutils.FakeGenerateKey))
		DeferCleanup(test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HVPA, true))

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:   seedName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
					Zones:  []string{"a", "b", "c"},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
				},
				DNS: gardencorev1beta1.SeedDNS{
					IngressDomain: pointer.String("someingress.example.com"),
				},
			},
		}

		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", seed.Name)

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())

			By("Forcefully remove finalizers")
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, seed))).To(Succeed())

			By("Ensure Seed is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
			}).Should(BeNotFoundError())

			By("Cleanup all labels/annotations from test namespace")
			patch := client.MergeFrom(testNamespace)
			testNamespace.Annotations = nil
			testNamespace.Labels = nil
			Expect(testClient.Patch(ctx, testNamespace, patch)).To(Succeed())
		})
	})

	Context("when seed namespace does not exist", func() {
		It("should not maintain the Bootstrapped condition", func() {
			By("Ensure Bootstrapped condition is not set")
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Status.Conditions
			}).Should(BeEmpty())
		})
	})

	Context("when seed namespace exists", func() {
		// Typically, GCM creates the seed-specific namespace, but it doesn't run in this test, hence we have to do it.
		var seedNamespace *corev1.Namespace

		JustBeforeEach(func() {
			By("Create seed namespace in garden")
			seedNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gutil.ComputeGardenNamespace(seed.Name)}}
			Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())

			By("Wait until the manager cache observes the seed namespace")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(seedNamespace), &corev1.Namespace{})
			}).Should(Succeed())

			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, seedNamespace)).To(Succeed())
			})

			By("Wait for Seed to have a cluster identity")
			Eventually(func(g Gomega) *string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Status.ClusterIdentity
			}).ShouldNot(BeNil())
		})

		Context("when internal domain secret does not exist", func() {
			It("should set the Bootstrapped condition to False", func() {
				By("Wait for Bootstrapped condition to be set to False")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					return seed.Status.Conditions
				}).Should(ContainCondition(
					OfType(gardencorev1beta1.SeedBootstrapped),
					WithStatus(gardencorev1beta1.ConditionFalse),
					WithReason("GardenSecretsError"),
				))
			})
		})

		Context("when internal domain secret exists", func() {
			JustBeforeEach(func() {
				By("Create internal domain secret in seed namespace")
				internalDomainSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    seedNamespace.Name,
					Labels: map[string]string{
						testID:                testRunID,
						"gardener.cloud/role": "internal-domain",
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "test",
						"dns.gardener.cloud/domain":   "example.com",
					},
				}}
				Expect(testClient.Create(ctx, internalDomainSecret)).To(Succeed())

				By("Wait until the manager cache observes the internal domain secret")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(internalDomainSecret), internalDomainSecret)
				}).Should(Succeed())

				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, internalDomainSecret)).To(Succeed())
				})
			})

			Context("when global monitoring secret does not exist", func() {
				It("should set the Bootstrapped condition to False", func() {
					By("Wait for Bootstrapped condition to be set to False")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						return seed.Status.Conditions
					}).Should(ContainCondition(
						OfType(gardencorev1beta1.SeedBootstrapped),
						WithStatus(gardencorev1beta1.ConditionFalse),
						WithReason("BootstrappingFailed"),
						WithMessage("global monitoring secret not found in seed namespace"),
					))
				})
			})

			Context("when global monitoring secret exists", func() {
				// Typically, GCM creates the global monitoring secret, but it doesn't run in this test, hence we have to do it.
				var globalMonitoringSecret *corev1.Secret

				JustBeforeEach(func() {
					DeferCleanup(
						test.WithVars(
							&resourcemanager.Until, untilInTest,
							&resourcemanager.TimeoutWaitForDeployment, 50*time.Millisecond,
						),
					)
					By("Create global monitoring secret in seed namespace")
					globalMonitoringSecret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "secret-",
							Namespace:    seedNamespace.Name,
							Labels: map[string]string{
								testID:                testRunID,
								"gardener.cloud/role": "global-monitoring",
							},
						},
						Data: map[string][]byte{"foo": []byte("bar")},
					}
					Expect(testClient.Create(ctx, globalMonitoringSecret)).To(Succeed())

					By("Wait until the manager cache observes the global monitoring secret")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(globalMonitoringSecret), globalMonitoringSecret)
					}).Should(Succeed())

					DeferCleanup(func() {
						Expect(testClient.Delete(ctx, globalMonitoringSecret)).To(Succeed())
					})
				})

				test := func(seedIsGarden bool) {
					By("Wait for Seed to have finalizer")
					Eventually(func(g Gomega) []string {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						return seed.Finalizers
					}).Should(ConsistOf("gardener"))

					By("Wait for Bootstrapped condition to be set to Progressing")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						return seed.Status.Conditions
					}).Should(ContainCondition(
						OfType(gardencorev1beta1.SeedBootstrapped),
						WithStatus(gardencorev1beta1.ConditionProgressing),
					))

					By("Verify that CA secret was generated")
					Eventually(func(g Gomega) []corev1.Secret {
						secretList := &corev1.SecretList{}
						g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"name": "ca-seed", "managed-by": "secrets-manager"})).To(Succeed())
						return secretList.Items
					}).Should(HaveLen(1))

					if !seedIsGarden {
						By("Verify that garden namespace was labeled and annotated appropriately")
						Eventually(func(g Gomega) {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
							g.Expect(testNamespace.Labels).To(And(
								HaveKeyWithValue("role", "garden"),
								HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
							))
							g.Expect(testNamespace.Annotations).To(And(
								HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
							))
						}).Should(Succeed())
					}

					By("Verify that kube-system namespace was labeled appropriately")
					Eventually(func(g Gomega) map[string]string {
						kubeSystemNamespace := &corev1.Namespace{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystemNamespace)).To(Succeed())
						return kubeSystemNamespace.Labels
					}).Should(HaveKeyWithValue("role", "kube-system"))

					By("Verify that global monitoring secret was replicated")
					Eventually(func(g Gomega) {
						secret := &corev1.Secret{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "seed-" + globalMonitoringSecret.Name, Namespace: testNamespace.Name}, secret)).To(Succeed())
						g.Expect(secret.Data).To(HaveKey("auth"))
					}).Should(Succeed())

					if !seedIsGarden {
						// The seed controller waits for the gardener-resource-manager Deployment to be healthy, so
						// let's fake this here.
						By("Patch gardener-resource-manager deployment to report healthiness")
						Eventually(func(g Gomega) {
							deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

							patch := client.MergeFrom(deployment.DeepCopy())
							deployment.Status.ObservedGeneration = deployment.Generation
							deployment.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
							g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
						}).Should(Succeed())

						// The gardener-resource-manager is not really running in this test scenario, hence there is
						// nothing to serve the webhook endpoints. However, the envtest kube-apiserver would try to
						// reach them, so let's better delete them here for the sake of this test.
						By("Delete gardener-resource-manager webhooks")
						mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager"}}
						validatingWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager"}}
						Eventually(func(g Gomega) {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhookConfiguration), mutatingWebhookConfiguration)).To(Succeed())
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhookConfiguration), validatingWebhookConfiguration)).To(Succeed())

							g.Expect(testClient.Delete(ctx, mutatingWebhookConfiguration)).To(Succeed())
							g.Expect(testClient.Delete(ctx, validatingWebhookConfiguration)).To(Succeed())
						}).Should(Succeed())
					} else {
						// Usually, the gardener-operator would deploy gardener-resource-manager and the related CRD for
						// ManagedResources. However, it is not really running, so we have to fake its behaviour here.
						By("Create CustomResourceDefinition for ManagedResources")
						var (
							applier = kubernetes.NewApplier(testClient, testClient.RESTMapper())
							obj     = kubernetes.NewManifestReader([]byte(managedResourcesCRD))
						)

						Expect(applier.ApplyManifest(ctx, obj, kubernetes.DefaultMergeFuncs)).To(Succeed())
						DeferCleanup(func() {
							Expect(applier.DeleteManifest(ctx, obj)).To(Succeed())
						})
					}

					By("Verify that the seed system components have been deployed")
					expectedManagedResources := []gomegatypes.GomegaMatcher{
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("cluster-autoscaler")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("cluster-identity")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("dependency-watchdog-endpoint")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("dependency-watchdog-probe")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("global-network-policies")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-state-metrics")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("system")})}),
					}

					if !seedIsGarden {
						expectedManagedResources = append(expectedManagedResources,
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("vpa")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("hvpa")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-druid")})}),
						)
					}

					Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
						managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
						g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
						return managedResourceList.Items
					}).
						// a lot of CPU-intensive stuff is happening between GRM deployment and this assertion, so to
						// prevent flakes we have to increase the timeout here manually
						WithTimeout(10 * time.Second).
						Should(ConsistOf(expectedManagedResources))

					By("Wait for Bootstrapped condition to be set to True")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						return seed.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.SeedBootstrapped), WithStatus(gardencorev1beta1.ConditionTrue)),
						ContainCondition(OfType(gardencorev1beta1.SeedSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionProgressing)),
					))

					By("Delete Seed")
					Expect(testClient.Delete(ctx, seed)).To(Succeed())

					if seedIsGarden {
						// The CRDs are cleaned up by the Destroy function of GRM. In case the seed is garden, the Destroy is called by the gardener-operator and since it's
						// not running in this test, we can safely assert the below-mentioned. But if the seed is not garden, it might so happen that, before we fetch the
						// ManagedResourceList and expect it to be empty, the CRDs are already gone. Since the gardener-resource-manager is deleted only after all the
						// managedresources are gone, we don't need to assert it separately.
						By("Verify that the seed system components have been deleted")
						Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
							managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
							g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
							return managedResourceList.Items
						}).Should(BeEmpty())
					} else {
						By("Verify that gardener-resource-manager has been deleted")
						Eventually(func(g Gomega) error {
							deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
							return testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
						}).Should(BeNotFoundError())
					}

					By("Ensure Seed is gone")
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
					}).Should(BeNotFoundError())
				}

				It("should properly maintain the Bootstrapped condition and deploy all seed system components", func() {
					test(false)
				})

				Context("when seed cluster is garden cluster at the same time", func() {
					BeforeEach(func() {
						By("Create Garden")
						garden := &operatorv1alpha1.Garden{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: "garden-",
								Labels: map[string]string{
									testID: testRunID,
								},
							},
							Spec: operatorv1alpha1.GardenSpec{
								VirtualCluster: operatorv1alpha1.VirtualCluster{
									Maintenance: operatorv1alpha1.Maintenance{
										TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
											Begin: "220000+0100",
											End:   "230000+0100",
										},
									},
								},
							},
						}
						Expect(testClient.Create(ctx, garden)).To(Succeed())
						log.Info("Created Garden for test", "garden", garden.Name)

						By("Wait until the manager cache observes the garden")
						Eventually(func() error {
							return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), &operatorv1alpha1.Garden{})
						}).Should(Succeed())

						DeferCleanup(func() {
							By("Delete Garden")
							Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())

							By("Wait until the manager cache observes garden deletion")
							Eventually(func() error {
								return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), &operatorv1alpha1.Garden{})
							}).Should(BeNotFoundError())
						})
					})

					It("should not manage components managed by gardener-operator", func() {
						test(true)
					})
				})
			})
		})
	})
})

func untilInTest(_ context.Context, _ time.Duration, _ retry.Func) error {
	return nil
}
