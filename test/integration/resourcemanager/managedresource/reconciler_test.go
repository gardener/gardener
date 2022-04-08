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
	"context"
	"encoding/json"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerv1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	secretName      = "test-secret"
	configMapName   = "test-configmap"
	newResourceName = "update-test-secret"
)

var _ = Describe("ManagedResource controller tests", func() {
	var (
		ctx                      = context.Background()
		secretForManagedResource *corev1.Secret
		managedResource          *resourcesv1alpha1.ManagedResource
		configMap                *corev1.ConfigMap
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
				// finalizer is added so this resource can not be deleted once we remove it from the MangedResource
				// referenced secret meanwhile adding a new resource in the refrenced secret set the ResourceApplied
				// condition to progressing and stuck at this untill we remove finalizer from the old resource
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
	})

	Context("#Reconciliation Modes/Annotations", func() {
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

		Context("#Ignore Mode", func() {
			It("should remove the resource from the ManagedResource status", func() {
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
				}, time.Minute, time.Second).Should(Succeed())
			})
		})

		Context("#Delete On Invalid Update", func() {
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
				}, time.Minute, time.Second).Should(Succeed())
			})
		})

		Context("#Keep Object", func() {
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
				}, time.Minute, time.Second).Should(Succeed())
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

		Context("#Ignore", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})

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

			It("should not revert any manual update on reosurce managed by ManagedResource", func() {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Or(Succeed(), BeNoMatchError()))

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
				}, time.Minute, time.Second).Should(BeTrue())
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
