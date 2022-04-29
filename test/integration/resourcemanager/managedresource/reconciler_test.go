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

package reconciler_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerv1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	secretName      = "test-secret"
	configMapName   = "test-configmap"
	newResourceName = "update-test-secret"
	deploymentName  = "test-deploy"
)

var _ = Describe("ManagedResource controller tests", func() {
	var (
		secretForManagedResource *corev1.Secret
		managedResource          *resourcesv1alpha1.ManagedResource
		configMap                *corev1.ConfigMap
		defaultPodTemplateSpec   *corev1.PodTemplateSpec
		deployment               *appsv1.Deployment
	)

	Context("create, update and delete operations", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				}, ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				}}

			data, err := createSecretDataFromObject(configMap, "config-map.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeNil())

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespaceName,
				},
				Data: data,
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: namespaceName,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(filter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
			}, time.Minute, 5*time.Second).Should(Succeed())

			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		})

		Context("create managed resource", func() {
			It("should successfully create the resources and maintain proper status conditions", func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}, time.Minute, time.Second).Should(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should fail to create the resource due to missing secret reference", func() {
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionFalse && condition.Reason == "CannotReadSecret"
				}, 30*time.Second, time.Second).Should(BeTrue())
			})

			It("should fail to create the resource due to incorrect object", func() {
				configMap := &corev1.ConfigMap{}
				data, err := createSecretDataFromObject(configMap, "config-map.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())
				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionFalse
				}, 30*time.Second, time.Second).Should(BeTrue())
			})

			It("should correctly set the condition ResourceApplied to Progressing", func() {
				// this finalizer is added to prolong the deletion of the resource so that we can
				// observe the controller successfully setting ResourceApplied condition to Progressing
				configMap.Finalizers = append(configMap.Finalizers, "kubernetes")
				data, err := createSecretDataFromObject(configMap, "config-map.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())

				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				newConfigMapName := "new-configmap"
				newConfigMap := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "ConfigMap",
					}, ObjectMeta: metav1.ObjectMeta{
						Name:      newConfigMapName,
						Namespace: namespaceName,
					}}

				newData, err := createSecretDataFromObject(newConfigMap, fmt.Sprintf("%s.yaml", newConfigMapName))
				Expect(err).ToNot(HaveOccurred())
				Expect(newData).ToNot(BeNil())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data = newData
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionProgressing
				}, time.Minute, time.Second).Should(BeTrue())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Or(Succeed(), BeNoMatchError()))
				configMap.Finalizers = []string{}
				Expect(testClient.Update(ctx, configMap)).To(Succeed())
			})
		})

		Context("update managed resource", func() {
			var (
				newResource        *corev1.Secret
				newResourceData    []byte
				newResourceDataKey string
			)

			BeforeEach(func() {
				newResource = &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      newResourceName,
						Namespace: namespaceName,
					},
					Data: map[string][]byte{
						"entry": []byte("value"),
					},
					Type: corev1.SecretTypeOpaque,
				}
				var err error
				newResourceData, err = json.Marshal(newResource)
				Expect(err).NotTo(HaveOccurred())
				Expect(newResourceData).NotTo(BeNil())
				newResourceDataKey = fmt.Sprintf("%s.yaml", newResourceName)
			})

			JustBeforeEach(func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should successfully create a new resource when the secret referenced by the managed resource is updated with data containing the new resource", func() {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				updatedSecretForManagedResource := secretForManagedResource.DeepCopy()
				updatedSecretForManagedResource.Data[newResourceDataKey] = newResourceData
				Expect(testClient.Update(ctx, updatedSecretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)).To(Succeed())
				}, time.Minute, 5*time.Second).Should(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should successfully update the managed resource with a new secret reference", func() {
				newConfigMapName := "new-configmap"
				newConfigMap := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "ConfigMap",
					}, ObjectMeta: metav1.ObjectMeta{
						Name:      newConfigMapName,
						Namespace: namespaceName,
					}}

				data, err := createSecretDataFromObject(newConfigMap, fmt.Sprintf("%s.yaml", newConfigMapName))
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())

				newSecretForManagedResource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "new-secret",
						Namespace: namespaceName,
					},
					Data: data,
				}

				managedResource.Spec.SecretRefs = append(managedResource.Spec.SecretRefs, corev1.LocalObjectReference{Name: newSecretForManagedResource.Name})
				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

				Expect(testClient.Create(ctx, newSecretForManagedResource)).To(Succeed())
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(newConfigMap), newConfigMap)).To(Succeed())
				}, time.Minute, 5*time.Second).Should(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should fail to update the managed resource if a new incorrect resource is added to the secret referenced by the managed resource", func() {
				newResource.TypeMeta = metav1.TypeMeta{}
				newResourceData, err := json.Marshal(newResource)
				Expect(err).NotTo(HaveOccurred())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data[newResourceDataKey] = newResourceData
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionFalse
				}, time.Minute, time.Second).Should(BeTrue())
			})
		})

		Context("delete managed resource", func() {
			BeforeEach(func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.SetFinalizers([]string{"test-finalizer"})
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())
			})

			JustAfterEach(func() {
				patch := client.MergeFrom(managedResource.DeepCopy())
				controllerutil.RemoveFinalizer(managedResource, "test-finalizer")
				Expect(testClient.Patch(ctx, managedResource, patch))
			})

			It("should set ManagedResource to unhealthy", func() {
				Expect(testClient.Delete(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionDeletionPending)),
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionDeletionPending)),
				)
			})
		})
	})

	Context("Resource class", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				}, ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				}}

			data, err := createSecretDataFromObject(configMap, "config-map.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeNil())

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespaceName,
				},
				Data: data,
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: namespaceName,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String("test"),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
			}, time.Minute, 5*time.Second).Should(Succeed())

			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		})

		It("should not reconcile ManagedResource of any other class except the default class", func() {
			Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())

			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(BeNotFoundError())

			Consistently(func(g Gomega) bool {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
				return condition == nil
			}).Should(BeTrue())
		})
	})

	Context("Reconciliation Modes/Annotations", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				}, ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				}}

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespaceName,
				},
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: namespaceName,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(filter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}

			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				}, time.Minute, 5*time.Second).Should(Succeed())

				Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		Context("Ignore Mode", func() {
			It("should not update/re-apply resources having ignore mode annotation and remove them from the ManagedResource status", func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore})

				data, err := createSecretDataFromObject(configMap, "config-map.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())

				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
				}).Should(Succeed())
			})
		})

		Context("Delete On Invalid Update", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"})

				data, err := createSecretDataFromObject(configMap, "config-map.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())

				secretForManagedResource.Data = data
			})

			JustBeforeEach(func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should not delete the resource on valid update", func() {
				configMap.Data = map[string]string{"foo": "bar"}
				newResourceData, err := json.Marshal(configMap)
				Expect(err).NotTo(HaveOccurred())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data["config-map.yaml"] = newResourceData
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				}, time.Minute, 5*time.Second).Should(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should delete the resource on invalid update", func() {
				newConfigMap := configMap
				newConfigMap.TypeMeta = metav1.TypeMeta{}
				newResourceData, err := json.Marshal(newConfigMap)
				Expect(err).NotTo(HaveOccurred())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data["config-map.yaml"] = newResourceData
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionFalse
				}, time.Minute, time.Second).Should(BeTrue())

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
				}).Should(Succeed())
			})
		})

		Context("Keep Object", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.KeepObject: "true"})

				data, err := createSecretDataFromObject(configMap, "config-map.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())

				secretForManagedResource.Data = data
			})

			JustBeforeEach(func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should keep the object in case it is removed from the MangedResource", func() {
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{}
				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				}).Should(Succeed())
			})

			It("should keep the object even after deletion of ManagedResource", func() {
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				}, time.Minute, 5*time.Second).Should(Succeed())

				Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			})
		})

		Context("Ignore", func() {
			It("should not revert any manual update on resource managed by ManagedResource when resource itself has ignore annotation", func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})

				data, err := createSecretDataFromObject(configMap, "config-map.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())

				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())

				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Update(ctx, configMap)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Consistently(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data != nil && configMap.Data["foo"] == "bar"
				}).Should(BeTrue())
			})

			It("should not revert any manual update on the resources when the ManagedResource has ignore annotation", func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				managedResource.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Update(ctx, configMap)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Consistently(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data != nil && configMap.Data["foo"] == "bar"
				}).Should(BeTrue())
			})
		})
	})

	Context("Preserve Replica/Resource", func() {
		BeforeEach(func() {
			defaultPodTemplateSpec = &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"foo": "bar"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "foo-container",
							Image: "foo",
						},
					},
				},
			}

			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespaceName,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Replicas: pointer.Int32Ptr(1),
					Template: *defaultPodTemplateSpec,
				},
			}

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespaceName,
				},
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: namespaceName,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(filter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

			// wait for finalizer to be added
			Eventually(func(g Gomega) bool {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				return deployment.Finalizers != nil && len(deployment.Finalizers) == 1
			}, time.Minute, time.Second).Should(BeTrue())

			// remove finalizer so the deployment can be deleted
			Expect(controllerutils.PatchRemoveFinalizers(ctx, testClient, deployment, "foregroundDeletion")).To(BeNil())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			}, time.Minute, 5*time.Second).Should(Succeed())

			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		})

		Context("Preserve Replicas", func() {
			It("should not preserve changes in the number of replicas if the resource don't have preserve-replicas annotation", func() {
				data, err := createSecretDataFromObject(deployment, "deployment.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())
				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Replicas = pointer.Int32Ptr(5)
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return *deployment.Spec.Replicas == int32(1)
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should preserve changes in the number of replicas if the resource has preserve-replicas annotation", func() {
				deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveReplicas: "true"})
				data, err := createSecretDataFromObject(deployment, "deployment.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())
				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Replicas = pointer.Int32Ptr(5)
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Consistently(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return *deployment.Spec.Replicas == int32(5)
				}).Should(BeTrue())
			})
		})

		Context("Preserve Resources", func() {
			var (
				newPodTemplateSpec *corev1.PodTemplateSpec
			)
			BeforeEach(func() {
				defaultPodTemplateSpec.Spec.Containers[0].Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("25m"),
						corev1.ResourceMemory: resource.MustParse("25Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				}

				deployment.Spec.Template = *defaultPodTemplateSpec

				newPodTemplateSpec = defaultPodTemplateSpec.DeepCopy()
				newPodTemplateSpec.Spec.Containers[0].Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("35m"),
						corev1.ResourceMemory: resource.MustParse("35Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("60m"),
						corev1.ResourceMemory: resource.MustParse("60Mi"),
					},
				}
			})

			It("should not preserve changes in resource requests and limits in Pod if the resource doesn't have preserve-resources annotation", func() {
				data, err := createSecretDataFromObject(deployment, "deployment.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())
				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template = *newPodTemplateSpec
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return compareResource(deployment.Spec.Template.Spec.Containers[0].Resources, defaultPodTemplateSpec.Spec.Containers[0].Resources)
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should preserve changes in resource requests and limits in Pod if the resource has preserve-resources annotation", func() {
				deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveResources: "true"})
				data, err := createSecretDataFromObject(deployment, "deployment.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())
				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template = *newPodTemplateSpec
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Consistently(func(g Gomega) bool {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return compareResource(deployment.Spec.Template.Spec.Containers[0].Resources, defaultPodTemplateSpec.Spec.Containers[0].Resources)
				}).Should(BeFalse())
			})
		})
	})
})

func createSecretDataFromObject(obj runtime.Object, key string) (map[string][]byte, error) {
	jsonObject, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{key: jsonObject}, nil
}

func compareResource(oldResource corev1.ResourceRequirements, newResource corev1.ResourceRequirements) bool {
	if *oldResource.Requests.Cpu() != *newResource.Requests.Cpu() {
		return false
	}
	if *oldResource.Requests.Memory() != *newResource.Requests.Memory() {
		return false
	}
	if *oldResource.Limits.Cpu() != *newResource.Limits.Cpu() {
		return false
	}
	if *oldResource.Limits.Memory() != *newResource.Limits.Memory() {
		return false
	}
	return true
}

func containCondition(matchers ...gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return ContainElement(And(matchers...))
}

func ofType(conditionType gardencorev1beta1.ConditionType) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Type": Equal(conditionType),
	})
}

func withStatus(status gardencorev1beta1.ConditionStatus) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Status": Equal(status),
	})
}

func withReason(reason string) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Reason": Equal(reason),
	})
}
