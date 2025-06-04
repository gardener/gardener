// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	managedresourcesutils "github.com/gardener/gardener/pkg/utils/managedresources"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ManagedResource controller tests", func() {
	const (
		dataKey       = "configmap.yaml"
		testFinalizer = "resources.gardener.cloud/test-finalizer"
	)

	var (
		// resourceName is used as a base name for all resources in the current test case
		resourceName string
		objectKey    client.ObjectKey

		secretForManagedResource *corev1.Secret
		managedResource          *resourcesv1alpha1.ManagedResource

		configMap       *corev1.ConfigMap
		signatureSecret *corev1.Secret
	)

	BeforeEach(func() {
		// use unique resource names specific to test case
		// If we use the same names for all test cases, failing cases will cause cascading failures because of AlreadyExistsErrors.
		// We want to avoid cascading failures because they are distracting from the actual root failure, and we want to
		// test all cases in a clean environment to make sure we don't have any unintended dependencies between test code.
		// We could also use GenerateName for this purpose, but then we would need an extra call to update the name of the
		// marshalled ConfigMap in the already created Secret.
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		objectKey = client.ObjectKey{Namespace: testNamespace.Name, Name: resourceName}

		configMap = &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Data: map[string]string{"abc": "xyz"},
		}

		signatureSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedresourcesutils.SigningSaltSecretName,
				Namespace: managedresourcesutils.SigningSaltSecretNamespace,
			},
			Data: map[string][]byte{
				managedresourcesutils.SigningSaltSecretKey: []byte("test-salt"),
			},
		}
		Expect(testClient.Create(ctx, signatureSecret)).To(Succeed())

		secretForManagedResource = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Data: secretDataForObject(configMap, dataKey),
		}

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:      ptr.To(filter.ResourceClass()),
				SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
			},
		}

		fakeClock.SetTime(time.Now())
	})

	JustBeforeEach(func() {
		if secretForManagedResource != nil {
			By("Create Secret for test")
			log.Info("Create Secret for test", "secret", objectKey)
			signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
			Expect(err).ToNot(HaveOccurred())
			secretForManagedResource.Annotations = map[string]string{
				managedresourcesutils.SignatureAnnotationKey: signature,
			}
			secretForManagedResource.ResourceVersion = ""
			Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
		}

		By("Create ManagedResource for test")
		log.Info("Create ManagedResource for test", "managedResource", objectKey)
		Expect(testClient.Create(ctx, managedResource)).To(Succeed())
	})

	AfterEach(func() {
		By("Delete ManagedResource")
		Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, objectKey, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError(), "ManagedResource should get released")
			g.Expect(testClient.Get(ctx, objectKey, &corev1.ConfigMap{})).To(BeNotFoundError(), "managed ConfigMap should get deleted")
		}).Should(Succeed())

		if secretForManagedResource != nil {
			By("Delete Secret")
			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		}
		if signatureSecret != nil {
			By("Delete Signature Secret")
			Expect(testClient.Delete(ctx, signatureSecret)).To(Or(Succeed(), BeNotFoundError()))
		}
	})

	Describe("create managed resource", func() {
		Describe("successful creation", func() {
			test := func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: testNamespace.Name,
					},
				}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}).Should(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			}

			Context("with uncompressed data", func() {
				It("should successfully create the resources and maintain proper status conditions", func() {
					test()
				})
			})

			Context("with compressed data", func() {
				BeforeEach(func() {
					compressedData, err := testutils.BrotliCompression(secretForManagedResource.Data[dataKey])
					Expect(err).ToNot(HaveOccurred())

					secretForManagedResource.Data[dataKey+".br"] = compressedData
					delete(secretForManagedResource.Data, dataKey)
				})

				It("should successfully create the resources and maintain proper status conditions", func() {
					test()
				})
			})
		})

		Context("missing secret", func() {
			BeforeEach(func() {
				secretForManagedResource = nil
			})

			It("should fail to create the resource due to missing secret reference", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("CannotReadSecret")),
				)
			})
		})

		Context("missing TypeMeta in object", func() {
			BeforeEach(func() {
				newConfigMap := &corev1.ConfigMap{}
				secretForManagedResource.Data = secretDataForObject(newConfigMap, dataKey)

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

			})

			It("should fail to create the resource due to incorrect object", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionFalse), WithReason(resourcesv1alpha1.ConditionApplyFailed)),
				)
			})
		})
	})

	Describe("update managed resource", func() {
		const newDataKey = "secret.yaml"
		var newResource *corev1.Secret

		BeforeEach(func() {
			newResource = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName + "-new",
					Namespace: testNamespace.Name,
				},
				Data: map[string][]byte{
					"entry":  []byte("value"),
					"entry2": []byte("value2"),
					"entry3": []byte("value3"),
				},
				Type: corev1.SecretTypeOpaque,
			}
		})

		JustBeforeEach(func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
			)

			// health controller is not running in this integration test
			// however, we want to make sure the controller correctly transitions Healthy/Progressing to Unknown,
			// hence we fake the health controller by manually patching the status
			patch := client.MergeFrom(managedResource.DeepCopy())
			oldConditions := managedResource.DeepCopy().Status.Conditions
			managedResource.Status.Conditions = v1beta1helper.MergeConditions(
				oldConditions,
				v1beta1helper.UpdatedConditionWithClock(fakeClock, v1beta1helper.GetOrInitConditionWithClock(fakeClock, oldConditions, resourcesv1alpha1.ResourcesHealthy), gardencorev1beta1.ConditionTrue, "test", "test"),
				v1beta1helper.UpdatedConditionWithClock(fakeClock, v1beta1helper.GetOrInitConditionWithClock(fakeClock, oldConditions, resourcesv1alpha1.ResourcesProgressing), gardencorev1beta1.ConditionFalse, "test", "test"),
			)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		Describe("resource set changes", func() {
			BeforeEach(func() {
				// this finalizer is added to prolong the deletion of the resource so that we can
				// observe the controller successfully setting ResourceApplied condition to Progressing
				controllerutil.AddFinalizer(configMap, testFinalizer)
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			It("should correctly set the condition ResourceApplied to Progressing", func() {
				newConfigMap := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName + "-new",
						Namespace: testNamespace.Name,
					},
				}

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data = secretDataForObject(newConfigMap, dataKey)
				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}
				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason(resourcesv1alpha1.ConditionDeletionPending)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
				))

				patch = client.MergeFrom(configMap.DeepCopy())
				controllerutil.RemoveFinalizer(configMap, testFinalizer)
				Expect(testClient.Patch(ctx, configMap, patch)).To(Succeed())
			})
		})

		Describe("resource data changes", func() {
			It("should set conditions to Unknown", func() {
				checksumBefore := managedResource.Status.SecretsDataChecksum
				Expect(checksumBefore).NotTo(BeNil())

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				configMap.Data = map[string]string{"foo": "bar"}
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}
				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					g.Expect(managedResource.Status.SecretsDataChecksum).NotTo(Equal(checksumBefore))
					return managedResource.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
				))

				Consistently(func(g Gomega) map[string]string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data
				}).Should(HaveKeyWithValue("foo", "bar"))
			})
		})

		Describe("new resource added", func() {
			It("should set conditions to Unknown", func() {
				checksumBefore := managedResource.Status.SecretsDataChecksum
				Expect(checksumBefore).NotTo(BeNil())

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data[newDataKey] = secretDataForObject(newResource, newDataKey)[newDataKey]

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					g.Expect(managedResource.Status.SecretsDataChecksum).NotTo(Equal(checksumBefore))
					return managedResource.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
				))

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)).To(Succeed())
				}).Should(Succeed())
			})

			It("should fail to update the managed resource if a new incorrect resource is added", func() {
				newResource.TypeMeta = metav1.TypeMeta{}

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data[newDataKey] = secretDataForObject(newResource, newDataKey)[newDataKey]

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionFalse), WithReason(resourcesv1alpha1.ConditionApplyFailed)),
				)
			})
		})

		Describe("new secret reference", func() {
			It("should successfully update the managed resource with a new secret reference", func() {
				checksumBefore := managedResource.Status.SecretsDataChecksum
				Expect(checksumBefore).NotTo(BeNil())

				newSecretForManagedResource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName + "-new-resource",
						Namespace: testNamespace.Name,
					},
					Data: secretDataForObject(newResource, dataKey),
				}

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, newSecretForManagedResource.Data)
				Expect(err).To(BeNil())
				newSecretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

				Expect(testClient.Create(ctx, newSecretForManagedResource)).To(Succeed())

				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.Spec.SecretRefs = append(managedResource.Spec.SecretRefs, corev1.LocalObjectReference{Name: newSecretForManagedResource.Name})
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					g.Expect(managedResource.Status.SecretsDataChecksum).NotTo(Equal(checksumBefore))
					return managedResource.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason(resourcesv1alpha1.ConditionChecksPending)),
				))

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)).To(Succeed())
				}).Should(Succeed())
			})
		})
	})

	Describe("delete managed resource", func() {
		JustBeforeEach(func() {
			// add finalizer to prolong deletion of ManagedResource after resource-manager removed its finalizer
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, managedResource)).To(Succeed())
				g.Expect(controllerutils.AddFinalizers(ctx, testClient, managedResource, testFinalizer)).To(Succeed())
			}).Should(Succeed())
		})

		JustAfterEach(func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, managedResource)).To(Succeed())
				g.Expect(controllerutils.RemoveFinalizers(ctx, testClient, managedResource, testFinalizer)).To(Succeed())
			}).Should(Succeed())
		})

		It("should set ManagedResource to unhealthy", func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
			)

			By("Delete ManagedResource")
			Expect(testClient.Delete(ctx, managedResource)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason(resourcesv1alpha1.ConditionDeletionPending)),
				ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionFalse), WithReason(resourcesv1alpha1.ConditionDeletionPending)),
				ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionDeletionPending)),
			))
		})
	})

	Describe("Resource class", func() {
		BeforeEach(func() {
			managedResource.Spec.Class = ptr.To("test")
		})

		It("should not reconcile ManagedResource of any other class except the default class", func() {
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(BeNotFoundError())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(BeEmpty())
		})
	})

	Describe("Reconciliation Modes/Annotations", func() {
		Describe("Ignore Mode", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			It("should not update/re-apply resources having ignore mode annotation and remove them from the ManagedResource status", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}).Should(BeNotFoundError())
			})
		})

		Describe("Delete On Invalid Update", func() {
			var originalUID types.UID

			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"})
				// provoke invalid update by trying to update an immutable configmap's data
				configMap.Immutable = ptr.To(true)
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			JustBeforeEach(func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				cm := configMap.DeepCopy() // copy in order to not remove TypeMeta from configMap which we rely on later
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
				// use UID to distinguish between object instances with different names
				// i.e. if UID changed, the object got deleted and recreated, if not it wasn't deleted
				originalUID = cm.UID
				Expect(originalUID).NotTo(BeEmpty()) // sanity check
			})

			It("should not delete the resource on valid update", func() {
				metav1.SetMetaDataLabel(&configMap.ObjectMeta, "foo", "bar")

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Consistently(func(g Gomega) types.UID {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.UID
				}).Should(Equal(originalUID), "ConfigMap should not get deleted")
			})

			It("should delete the resource on invalid update", func() {
				configMap.Data = map[string]string{"invalid": "update"}

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) types.UID {
					// also accept transient NotFoundError because controller needs to delete the ConfigMap first before recreating it
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Or(Succeed(), BeNotFoundError()))
					return configMap.UID
				}).ShouldNot(Equal(originalUID), "ConfigMap should get deleted and recreated")
			})
		})

		Describe("Keep Object", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.KeepObject: "true"})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

			})

			JustBeforeEach(func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			JustAfterEach(func() {
				Expect(testClient.Delete(ctx, configMap)).To(Or(Succeed(), BeNotFoundError()))
			})

			It("should keep the object in case it is removed from the MangedResource", func() {
				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{}
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}).Should(Succeed())
			})

			It("should keep the object even after deletion of ManagedResource", func() {
				By("Delete ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
				}).Should(BeNotFoundError())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			})
		})

		Describe("Finalize Deletion", func() {
			finalizeDeletionAfter := time.Hour

			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.FinalizeDeletionAfter: finalizeDeletionAfter.String()})
				configMap.Finalizers = []string{"some.finalizer.to/make-the-deletion-stuck"}
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

			})

			JustBeforeEach(func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(ContainCondition(
					OfType(resourcesv1alpha1.ResourcesApplied),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason(resourcesv1alpha1.ConditionApplySucceeded),
				))
			})

			JustAfterEach(func() {
				patch := client.MergeFrom(configMap.DeepCopy())
				configMap.Finalizers = nil
				Expect(testClient.Patch(ctx, configMap, patch)).To(Or(Succeed(), BeNotFoundError()))
				Expect(testClient.Delete(ctx, configMap)).To(Or(Succeed(), BeNotFoundError()))
			})

			It("should forcefully finalize the deletion after the grace period has elapsed", func() {
				By("Delete ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Succeed())

				By("Stepping fake clock")
				fakeClock.Step(finalizeDeletionAfter - time.Second)

				By("Expect ConfigMap to remain in the system")
				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}).Should(Succeed())

				By("Stepping fake clock")
				fakeClock.Step(2 * time.Second)

				By("Expect ConfigMap to disappear from the system")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}).Should(BeNotFoundError())
			})
		})

		Describe("Keep garbage-collectable object", func() {
			var node *corev1.Node

			BeforeEach(func() {
				configMap.SetLabels(map[string]string{references.LabelKeyGarbageCollectable: references.LabelValueGarbageCollectable})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				node = &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node",
						Labels: map[string]string{references.LabelKeyGarbageCollectable: references.LabelValueGarbageCollectable},
					},
				}
				secretForManagedResource.Data["node.yaml"] = jsonDataForObject(node)

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

			})

			JustBeforeEach(func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			JustAfterEach(func() {
				Expect(testClient.Delete(ctx, configMap)).To(Or(Succeed(), BeNotFoundError()))
				Expect(testClient.Delete(ctx, node)).To(Or(Succeed(), BeNotFoundError()))
			})

			It("should keep the garbage-collectable objects in case it is removed from the MangedResource", func() {
				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{}
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}).Should(Succeed())

				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(node), node)
				}).Should(BeNotFoundError())
			})

			It("should keep the garbage-collectable objects even after deletion of ManagedResource", func() {
				By("Delete ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
				}).Should(BeNotFoundError())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(BeNotFoundError())
			})
		})

		Describe("Ignore on Resource", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

			})

			It("should not revert any manual update to managed resource", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				patch := client.MergeFrom(configMap.DeepCopy())
				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Patch(ctx, configMap, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Consistently(func(g Gomega) map[string]string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data
				}).Should(HaveKeyWithValue("foo", "bar"))
			})
		})

		Describe("Ignore on ManagedResource", func() {
			It("should not revert any manual update to the resources", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
				))

				patch = client.MergeFrom(configMap.DeepCopy())
				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Patch(ctx, configMap, patch)).To(Succeed())

				Consistently(func(g Gomega) map[string]string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data
				}).Should(HaveKeyWithValue("foo", "bar"))
			})
		})

		Describe("Ensure resources.gardener.cloud/managed-by label", func() {
			var (
				defaultPodTemplateSpec *corev1.PodTemplateSpec
				deployment             *appsv1.Deployment
			)

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
						Name:      resourceName,
						Namespace: testNamespace.Name,
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						Replicas: ptr.To[int32](1),
						Template: *defaultPodTemplateSpec,
					},
				}

				secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}
			})

			AfterEach(func() {
				By("Delete ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

				By("Delete Secret")
				Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))

				// resource-manager deletes Deployments with foreground deletion, which causes API server to add the
				// foregroundDeletion finalizer. It is removed by kube-controller-manager's garbage collector, which is not
				// running in envtest, so we me might need to remove it ourselves.
				Eventually(func(g Gomega) bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
					if apierrors.IsNotFound(err) {
						// deployment is gone, done
						return true
					}
					g.Expect(err).To(Succeed())
					// no point in checking whether finalizer is present, just try to remove it until Deployment is gone
					g.Expect(controllerutils.RemoveFinalizers(ctx, testClient, deployment, metav1.FinalizerDeleteDependents)).To(Succeed())
					return false
				}).Should(BeTrue())
			})

			Context("injected labels are not overlapping", func() {
				BeforeEach(func() {
					managedResource.Spec.InjectLabels = map[string]string{
						"foo": "bar",
					}
				})

				It("should confirm that the managed-by label is set", func() {
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
						return managedResource.Status.Conditions
					}).Should(
						ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
					)

					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					v, ok := deployment.Labels[resourcesv1alpha1.ManagedBy]
					Expect(ok).To(BeTrue())
					Expect(v).To(Equal("gardener"))
					v, ok = deployment.Spec.Template.Labels[resourcesv1alpha1.ManagedBy]
					Expect(ok).To(BeTrue())
					Expect(v).To(Equal("gardener"))

					// check that the other injected labels are also there
					v, ok = deployment.Labels["foo"]
					Expect(ok).To(BeTrue())
					Expect(v).To(Equal("bar"))
					v, ok = deployment.Spec.Template.Labels["foo"]
					Expect(ok).To(BeTrue())
					Expect(v).To(Equal("bar"))
				})
			})

			Context("injected labels are overlapping", func() {
				BeforeEach(func() {
					managedResource.Spec.InjectLabels = map[string]string{
						resourcesv1alpha1.ManagedBy: "foo",
					}
				})
				It("should confirm that the managed-by label is not overwritten by injected labels", func() {
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
						return managedResource.Status.Conditions
					}).Should(
						ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
					)

					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					v, ok := deployment.Labels[resourcesv1alpha1.ManagedBy]
					Expect(ok).To(BeTrue())
					Expect(v).To(Equal("gardener"))
					v, ok = deployment.Spec.Template.Labels[resourcesv1alpha1.ManagedBy]
					Expect(ok).To(BeTrue())
					Expect(v).To(Equal("gardener"))
				})
			})
		})

		Describe("Preserve Replica/Resource", func() {
			var (
				defaultPodTemplateSpec *corev1.PodTemplateSpec
				deployment             *appsv1.Deployment
			)

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
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("25m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
								},
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
						Name:      resourceName,
						Namespace: testNamespace.Name,
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						Replicas: ptr.To[int32](1),
						Template: *defaultPodTemplateSpec,
					},
				}

				secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

				signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
				Expect(err).To(BeNil())
				secretForManagedResource.Annotations = map[string]string{
					managedresourcesutils.SignatureAnnotationKey: signature,
				}

			})

			AfterEach(func() {
				By("Delete ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

				// resource-manager deletes Deployments with foreground deletion, which causes API server to add the
				// foregroundDeletion finalizer. It is removed by kube-controller-manager's garbage collector, which is not
				// running in envtest, so we me might need to remove it ourselves.
				Eventually(func(g Gomega) bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
					if apierrors.IsNotFound(err) {
						// deployment is gone, done
						return true
					}
					g.Expect(err).To(Succeed())
					// no point in checking whether finalizer is present, just try to remove it until Deployment is gone
					g.Expect(controllerutils.RemoveFinalizers(ctx, testClient, deployment, metav1.FinalizerDeleteDependents)).To(Succeed())
					return false
				}).Should(BeTrue())
			})

			Describe("Preserve Replicas", func() {
				Context("resource doesn't have preserve-replicas annotation", func() {
					It("should not preserve changes in the number of replicas", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Replicas = ptr.To[int32](5)
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						patch = client.MergeFrom(managedResource.DeepCopy())
						metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "gardener.cloud/operation", "reconcile")
						Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

						Eventually(func(g Gomega) int32 {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return *deployment.Spec.Replicas
						}).Should(BeEquivalentTo(1))
					})
				})

				Context("resource has preserve-replicas annotation", func() {
					BeforeEach(func() {
						deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveReplicas: "true"})
						secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

						signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
						Expect(err).To(BeNil())
						secretForManagedResource.Annotations = map[string]string{
							managedresourcesutils.SignatureAnnotationKey: signature,
						}

					})

					It("should preserve changes in the number of replicas if the resource", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Replicas = ptr.To[int32](5)
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						Consistently(func(g Gomega) int32 {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return *deployment.Spec.Replicas
						}).Should(BeEquivalentTo(5))
					})
				})
			})

			Describe("Preserve Resources", func() {
				var newPodTemplateSpec *corev1.PodTemplateSpec

				BeforeEach(func() {
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

				Context("resource doesn't have preserve-resources annotation", func() {
					It("should not preserve changes in resource requests and limits in Pod", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Template = *newPodTemplateSpec
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						patch = client.MergeFrom(managedResource.DeepCopy())
						metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "gardener.cloud/operation", "reconcile")
						Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

						Eventually(func(g Gomega) corev1.ResourceRequirements {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return deployment.Spec.Template.Spec.Containers[0].Resources
						}).Should(DeepEqual(defaultPodTemplateSpec.Spec.Containers[0].Resources))
					})
				})

				Context("resource has preserve-resources annotation", func() {
					BeforeEach(func() {
						deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveResources: "true"})
						secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

						signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
						Expect(err).To(BeNil())
						secretForManagedResource.Annotations = map[string]string{
							managedresourcesutils.SignatureAnnotationKey: signature,
						}
					})

					It("should preserve changes in resource requests and limits in Pod", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Template = *newPodTemplateSpec
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						Consistently(func(g Gomega) corev1.ResourceRequirements {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return deployment.Spec.Template.Spec.Containers[0].Resources
						}).Should(DeepEqual(newPodTemplateSpec.Spec.Containers[0].Resources))
					})
				})
			})
		})
	})

	Describe("Immutable resources", func() {
		BeforeEach(func() {
			configMap.Immutable = ptr.To(true)
			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
				},
				Data: secretDataForObject(configMap, dataKey),
			}

			signature, err := managedresourcesutils.CalculateSignature(ctx, testClient, secretForManagedResource.Data)
			Expect(err).ToNot(HaveOccurred())
			secretForManagedResource.Annotations = map[string]string{
				managedresourcesutils.SignatureAnnotationKey: signature,
			}
		})

		It("should recreate the resource if the update is invalid", func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue), WithReason(resourcesv1alpha1.ConditionApplySucceeded)),
			)

			Expect(testClient.Delete(ctx, configMap)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(BeNotFoundError())

			newData := map[string]string{
				"foo": "bar",
			}
			oldData := configMap.Data
			configMap.Data = newData
			Expect(testClient.Create(ctx, configMap)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(Succeed())

			Consistently(func(g Gomega) map[string]string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				return configMap.Data
			}).Should(Equal(newData))

			patch := client.MergeFrom(managedResource.DeepCopy())
			metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "gardener.cloud/operation", "reconcile")
			Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) map[string]string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				return configMap.Data
			}).Should(Equal(oldData))
		})
	})
})

func secretDataForObject(obj runtime.Object, key string) map[string][]byte {
	return map[string][]byte{key: jsonDataForObject(obj)}
}

func jsonDataForObject(obj runtime.Object) []byte {
	jsonObject, err := json.Marshal(obj)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return jsonObject
}
