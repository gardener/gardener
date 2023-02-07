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

package extensions_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ShootState Extensions controller tests", func() {
	var (
		testNamespace  *corev1.Namespace
		shootState     *gardencorev1beta1.ShootState
		cluster        *extensionsv1alpha1.Cluster
		infrastructure *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
				GenerateName: "garden-",
			},
		}
		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		shootState = &gardencorev1beta1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "shoot-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   testNamespace.Name,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{Raw: []byte(`{}`)},
				Seed:         runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		infrastructure = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "infra-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create ShootState")
		Expect(testClient.Create(ctx, shootState)).To(Succeed())
		log.Info("Created ShootState", "shootState", client.ObjectKeyFromObject(shootState))

		By("Wait until manager has observed ShootState")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)
		}).Should(Succeed())

		By("Create Cluster")
		cluster.Spec.Shoot.Object = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootState.Name,
				Namespace: shootState.Namespace,
			},
		}
		Expect(testClient.Create(ctx, cluster)).To(Succeed())
		log.Info("Created Cluster", "cluster", client.ObjectKeyFromObject(cluster))

		By("Wait until manager has observed Cluster")
		// Use the manager's cache to ensure it has observed the Cluster. Otherwise, the controller might simply not
		// sync the state of the extension resource. This should not happen in reality, so make sure to stabilize the
		// test and keep the controller simple. See https://github.com/gardener/gardener/issues/6923.
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
		}).Should(Succeed())

		By("Create Infrastructure")
		Expect(testClient.Create(ctx, infrastructure)).To(Succeed())
		log.Info("Created Infrastructure", "infrastructure", client.ObjectKeyFromObject(infrastructure))

		DeferCleanup(func() {
			By("Delete Infrastructure")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, infrastructure))).To(Succeed())

			By("Delete Cluster")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cluster))).To(Succeed())

			By("Delete ShootState")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootState))).To(Succeed())

			By("Wait for Infrastructure to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(infrastructure), infrastructure)
			}).Should(BeNotFoundError())

			By("Wait for Cluster to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
			}).Should(BeNotFoundError())

			By("Wait for ShootState to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)
			}).Should(BeNotFoundError())
		})
	})

	Context("when reconciliation should be performed", func() {
		It("should update the state", func() {
			By("Patch status.state in Infrastructure")
			patch := client.MergeFrom(infrastructure.DeepCopy())
			infrastructure.Status.State = &runtime.RawExtension{Raw: []byte(`{"some":"state"}`)}
			Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Wait for ShootState to reflect new status")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Extensions).To(ConsistOf(gardencorev1beta1.ExtensionResourceState{
					Kind:  "Infrastructure",
					Name:  &infrastructure.Name,
					State: infrastructure.Status.State,
				}))
			}).Should(Succeed())
		})

		It("should update the resources", func() {
			By("Create secrets to be referenced")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-secret",
					Namespace: infrastructure.Namespace,
					Labels:    map[string]string{testID: testRunID},
				},
				Data: map[string][]byte{"foo": []byte("bar")},
			}
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-configmap",
					Namespace: infrastructure.Namespace,
					Labels:    map[string]string{testID: testRunID},
				},
				Data: map[string]string{"foo": "bar"},
			}
			Expect(testClient.Create(ctx, configMap)).To(Succeed())

			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, configMap)).To(Succeed())
				Expect(testClient.Delete(ctx, secret)).To(Succeed())
			})

			By("Patch status.resources in Infrastructure to reference new secret")
			patch := client.MergeFrom(infrastructure.DeepCopy())
			infrastructure.Status.Resources = append(infrastructure.Status.Resources, gardencorev1beta1.NamedResourceReference{
				Name: "foo",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secret.Name,
				},
			})
			Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Wait for ShootState to reflect new status")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Extensions).To(ConsistOf(gardencorev1beta1.ExtensionResourceState{
					Kind:      "Infrastructure",
					Name:      &infrastructure.Name,
					Resources: infrastructure.Status.Resources,
				}))
				g.Expect(shootState.Spec.Resources).To(ConsistOf(gardencorev1beta1.ResourceData{
					CrossVersionObjectReference: infrastructure.Status.Resources[0].ResourceRef,
					Data:                        runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","data":{"foo":"YmFy"},"kind":"Secret","metadata":{"labels":{"` + testID + `":"` + testRunID + `"},"name":"` + secret.Name + `","namespace":"` + secret.Namespace + `"},"type":"Opaque"}`)},
				}))
			}).Should(Succeed())

			By("Update secret data")
			patch = client.MergeFrom(secret.DeepCopy())
			secret.Data["foo"] = []byte("baz")
			Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())

			By("Patch status.resources in Infrastructure to reference new ConfigMap")
			patch = client.MergeFrom(infrastructure.DeepCopy())
			infrastructure.Status.Resources = append(infrastructure.Status.Resources, gardencorev1beta1.NamedResourceReference{
				Name: "bar",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       configMap.Name,
				},
			})
			Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Wait for ShootState to reflect new status")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Extensions).To(ConsistOf(gardencorev1beta1.ExtensionResourceState{
					Kind:      "Infrastructure",
					Name:      &infrastructure.Name,
					Resources: infrastructure.Status.Resources,
				}))
				g.Expect(shootState.Spec.Resources).To(ConsistOf(
					gardencorev1beta1.ResourceData{
						CrossVersionObjectReference: infrastructure.Status.Resources[0].ResourceRef,
						Data:                        runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","data":{"foo":"YmF6"},"kind":"Secret","metadata":{"labels":{"` + testID + `":"` + testRunID + `"},"name":"` + secret.Name + `","namespace":"` + secret.Namespace + `"},"type":"Opaque"}`)},
					},
					gardencorev1beta1.ResourceData{
						CrossVersionObjectReference: infrastructure.Status.Resources[1].ResourceRef,
						Data:                        runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","data":{"foo":"bar"},"kind":"ConfigMap","metadata":{"labels":{"` + testID + `":"` + testRunID + `"},"name":"` + configMap.Name + `","namespace":"` + configMap.Namespace + `"}}`)},
					},
				))
			}).Should(Succeed())

			By("Patch status.resources in Infrastructure to nil")
			patch = client.MergeFrom(infrastructure.DeepCopy())
			infrastructure.Status.Resources = nil
			Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Wait for ShootState to reflect new status")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Extensions).To(BeEmpty())
				g.Expect(shootState.Spec.Resources).To(BeEmpty())
			}).Should(Succeed())
		})

		It("should remove the state when deletion timestamp is set", func() {
			By("Patch status.state in Infrastructure")
			patch := client.MergeFrom(infrastructure.DeepCopy())
			infrastructure.Status.State = &runtime.RawExtension{Raw: []byte(`{"some":"state"}`)}
			Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Wait for ShootState to reflect new status")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Extensions).To(ConsistOf(gardencorev1beta1.ExtensionResourceState{
					Kind:  "Infrastructure",
					Name:  &infrastructure.Name,
					State: infrastructure.Status.State,
				}))
			}).Should(Succeed())

			By("Add fake finalizer to Infrastructure to prolong deletion")
			patch = client.MergeFrom(infrastructure.DeepCopy())
			Expect(controllerutil.AddFinalizer(infrastructure, "foo.com/bar")).To(BeTrue())
			Expect(testClient.Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Delete Infrastructure")
			Expect(testClient.Delete(ctx, infrastructure)).To(Succeed())

			By("Patch status.state in Infrastructure to some new information")
			patch = client.MergeFrom(infrastructure.DeepCopy())
			infrastructure.Status.State = &runtime.RawExtension{Raw: []byte(`{"some":"new-state"}`)}
			Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Wait for ShootState to be updated (state completely removed)")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Extensions).To(BeEmpty())
			}).Should(Succeed())

			By("Remove fake finalizer from Infrastructure")
			patch = client.MergeFrom(infrastructure.DeepCopy())
			infrastructure.Finalizers = nil
			Expect(testClient.Patch(ctx, infrastructure, patch)).To(Succeed())

			By("Ensure ShootState was not updated anymore")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Extensions).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("when reconciliation should be skipped", func() {
		testForOperationAnnotation := func(operationAnnotation string) {
			It("should do nothing because of operation annotation "+operationAnnotation, func() {
				By("Patch status.state in Infrastructure")
				patch := client.MergeFrom(infrastructure.DeepCopy())
				infrastructure.Status.State = &runtime.RawExtension{Raw: []byte(`{"some":"state"}`)}
				Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())
				state := *infrastructure.Status.State

				By("Wait for ShootState to reflect new status")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					g.Expect(shootState.Spec.Extensions).To(ConsistOf(gardencorev1beta1.ExtensionResourceState{
						Kind:  "Infrastructure",
						Name:  &infrastructure.Name,
						State: &state,
					}))
				}).Should(Succeed())

				By("Add operation annotation")
				patch = client.MergeFrom(infrastructure.DeepCopy())
				metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, "gardener.cloud/operation", operationAnnotation)
				Expect(testClient.Patch(ctx, infrastructure, patch)).To(Succeed())

				By("Patch status.state in Infrastructure to some new information")
				patch = client.MergeFrom(infrastructure.DeepCopy())
				infrastructure.Status.State = &runtime.RawExtension{Raw: []byte(`{"some":"new-state"}`)}
				Expect(testClient.Status().Patch(ctx, infrastructure, patch)).To(Succeed())

				By("Ensure that ShootState was not updated")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					g.Expect(shootState.Spec.Extensions).To(ConsistOf(gardencorev1beta1.ExtensionResourceState{
						Kind:  "Infrastructure",
						Name:  &infrastructure.Name,
						State: &state,
					}))
				}).Should(Succeed())

				By("Remove operation annotation")
				patch = client.MergeFrom(infrastructure.DeepCopy())
				delete(infrastructure.Annotations, "gardener.cloud/operation")
				Expect(testClient.Patch(ctx, infrastructure, patch)).To(Succeed())

				By("Wait for ShootState to reflect new status")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					g.Expect(shootState.Spec.Extensions).To(ConsistOf(gardencorev1beta1.ExtensionResourceState{
						Kind:  "Infrastructure",
						Name:  &infrastructure.Name,
						State: infrastructure.Status.State,
					}))
				}).Should(Succeed())
			})
		}

		Context("wait-for-state", func() { testForOperationAnnotation("wait-for-state") })
		Context("restore", func() { testForOperationAnnotation("restore") })
		Context("migrate", func() { testForOperationAnnotation("migrate") })
	})
})
