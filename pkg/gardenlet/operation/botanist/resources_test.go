// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"bytes"
	"context"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Resources", func() {
	var (
		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		botanist *Botanist

		ctx                   = context.TODO()
		gardenNamespace       = "garden-foo"
		controlPlaneNamespace = "shoot--foo--bar"

		secret           *corev1.Secret
		workloadIdentity *securityv1alpha1.WorkloadIdentity
	)

	BeforeEach(func() {
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

		botanist = &Botanist{Operation: &operation.Operation{
			GardenClient:  gardenClient,
			SeedClientSet: seedClientSet,
			Shoot:         &shoot.Shoot{ControlPlaneNamespace: controlPlaneNamespace},
		}}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   gardenNamespace,
				Finalizers:  []string{"some-finalizer1", "some-finalizer2"},
				Annotations: map[string]string{"foo": "bar"},
				Labels:      map[string]string{"bar": "foo"},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"some": []byte("data")},
		}

		workloadIdentity = &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wi-foo",
				Namespace: gardenNamespace,
			},
			Spec: securityv1alpha1.WorkloadIdentitySpec{
				TargetSystem: securityv1alpha1.TargetSystem{
					Type: "test",
				},
			},
		}

		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "core.gardener.cloud/v1beta1",
				Kind:       "Shoot",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gardenNamespace,
				Name:      "bar",
				UID:       "shoot-uuid",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Resources: []gardencorev1beta1.NamedResourceReference{
					{
						Name: "resource-secret",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secret.Name,
						},
					},
					{
						Name: "resource-workload-identity",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "security.gardener.cloud/v1alpha1",
							Kind:       "WorkloadIdentity",
							Name:       workloadIdentity.Name,
						},
					},
				},
			},
		})
	})

	Describe("#DeployReferencedResources", func() {
		expectReferencedResourcesInSeed := func(expectedMRObjects, expectedWorkloadIdentityObjects []client.Object) {
			managedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: "referenced-resources"}, managedResource)).To(Succeed())
			Expect(managedResource.Spec.Class).To(PointTo(Equal("seed")))
			Expect(managedResource.Spec.ForceOverwriteAnnotations).To(PointTo(BeFalse()))
			Expect(managedResource.Spec.KeepObjects).To(PointTo(BeFalse()))
			Expect(managedResource.Spec.SecretRefs).To(HaveLen(1))

			managedResourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResource.Spec.SecretRefs[0].Name,
					Namespace: managedResource.Namespace,
				},
			}
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Data).To(HaveKey("referenced-resources"))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			var (
				decoder    = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(managedResourceSecret.Data["referenced-resources"]), 1024)
				decodedObj map[string]any
			)

			var i = 0
			for indexInFile := 0; true; indexInFile++ {
				err := decoder.Decode(&decodedObj)
				if err == io.EOF {
					break
				}
				Expect(err).NotTo(HaveOccurred())

				if decodedObj == nil {
					continue
				}

				Expect(i).To(BeNumerically("<", len(expectedMRObjects)), "managed resource secret should contain only %d objects", len(expectedMRObjects))

				actualObject := expectedMRObjects[i].DeepCopyObject()
				Expect(kubernetesscheme.Scheme.Convert(&unstructured.Unstructured{Object: decodedObj}, actualObject, nil)).To(Succeed())
				Expect(actualObject).To(DeepEqual(expectedMRObjects[i]))

				i++
			}

			for _, expectedObj := range expectedWorkloadIdentityObjects {
				actualObj := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(expectedObj), actualObj)).To(Succeed())
				Expect(actualObj).To(DeepEqual(expectedObj))
			}
		}

		It("should deploy the managed resource and its secret for the referenced resources", func() {
			Expect(gardenClient.Create(ctx, secret)).To(Succeed())
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

			Expect(botanist.DeployReferencedResources(ctx)).To(Succeed())

			expectReferencedResourcesInSeed(
				[]client.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ref-" + secret.Name,
							Namespace: controlPlaneNamespace,
						},
						Type: secret.Type,
						Data: secret.Data,
					},
				},
				[]client.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"garden-foo","uid":"shoot-uuid"}`,
								"workloadidentity.security.gardener.cloud/name":           "wi-foo",
								"workloadidentity.security.gardener.cloud/namespace":      "garden-foo",
							},
							Labels: map[string]string{
								"security.gardener.cloud/purpose":                     "workload-identity-token-requestor",
								"workloadidentity.security.gardener.cloud/referenced": "true",
								"workloadidentity.security.gardener.cloud/provider":   "test",
							},
							Name:            "workload-identity-ref-" + workloadIdentity.Name,
							Namespace:       controlPlaneNamespace,
							ResourceVersion: "1",
						},
						Type: corev1.SecretTypeOpaque,
					},
				},
			)
		})
	})

	Describe("#DestroyReferencedResources", func() {
		It("should destroy the managed resource and its secret for the referenced resources", func() {
			managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "referenced-resources"}}
			Expect(seedClient.Create(ctx, managedResource)).To(Succeed())

			managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "referenced-resources"}}
			Expect(seedClient.Create(ctx, managedResourceSecret)).To(Succeed())

			workloadIdentitySecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: controlPlaneNamespace,
					Name:      "workload-identity-ref-foo",
					Labels: map[string]string{
						"workloadidentity.security.gardener.cloud/referenced": "true",
					},
				},
			}
			Expect(seedClient.Create(ctx, workloadIdentitySecret)).To(Succeed())

			Expect(botanist.DestroyReferencedResources(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedResource), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), &corev1.Secret{})).To(BeNotFoundError())

			workloadIdentitySecrets := &corev1.SecretList{}
			Expect(seedClient.List(ctx, workloadIdentitySecrets,
				client.InNamespace(gardenNamespace),
				client.MatchingLabels(map[string]string{
					"workloadidentity.security.gardener.cloud/referenced": "true",
				}),
			)).To(Succeed())
			Expect(workloadIdentitySecrets.Items).To(BeEmpty())
		})
	})

	Describe("#PopulateStaticManifestsFromSeedToShoot", func() {
		var (
			managedResourceName = "static-manifests-propagated-from-seed"
			secretNamePrefix    = "static-manifests-"
			shootLabels         = map[string]string{"environment": "production", "purpose": "testing"}
		)

		BeforeEach(func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: "garden",
					Labels:    shootLabels,
				},
			})
		})

		Context("when there are no secrets with static manifests label", func() {
			It("should delete the managed resource if it exists", func() {
				managedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: controlPlaneNamespace,
					},
				}
				Expect(seedClient.Create(ctx, managedResource)).To(Succeed())

				Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedResource), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			})

			It("should not fail if managed resource does not exist", func() {
				Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())
			})
		})

		Context("when there are secrets with static manifests label", func() {
			var secret1, secret2, secret3 *corev1.Secret

			BeforeEach(func() {
				secret1 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "manifest-1",
						Namespace: "garden",
						Labels:    map[string]string{"shoot.gardener.cloud/static-manifests": "true"},
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"manifest": []byte("data1")},
				}

				secret2 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "manifest-2",
						Namespace: "garden",
						Labels:    map[string]string{"shoot.gardener.cloud/static-manifests": "true"},
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{"tls.crt": []byte("cert"), "tls.key": []byte("key")},
				}

				secret3 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "manifest-3",
						Namespace: "garden",
						Labels:    map[string]string{"shoot.gardener.cloud/static-manifests": "true"},
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"manifest": []byte("data3")},
				}
			})

			It("should copy secrets to shoot control plane namespace with prefix", func() {
				Expect(seedClient.Create(ctx, secret1)).To(Succeed())
				Expect(seedClient.Create(ctx, secret2)).To(Succeed())

				Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

				copiedSecret1 := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, copiedSecret1)).To(Succeed())
				Expect(copiedSecret1.Type).To(Equal(secret1.Type))
				Expect(copiedSecret1.Data).To(Equal(secret1.Data))
				Expect(copiedSecret1.Immutable).To(Equal(ptr.To(false)))
				Expect(copiedSecret1.Labels).To(HaveKeyWithValue("shoot.gardener.cloud/static-manifests", "true"))

				copiedSecret2 := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret2.Name}, copiedSecret2)).To(Succeed())
				Expect(copiedSecret2.Type).To(Equal(secret2.Type))
				Expect(copiedSecret2.Data).To(Equal(secret2.Data))
			})

			It("should create a managed resource referencing all copied secrets", func() {
				Expect(seedClient.Create(ctx, secret1)).To(Succeed())
				Expect(seedClient.Create(ctx, secret2)).To(Succeed())

				Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

				managedResource := &resourcesv1alpha1.ManagedResource{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: managedResourceName}, managedResource)).To(Succeed())
				Expect(managedResource.Spec.SecretRefs).To(ConsistOf(
					corev1.LocalObjectReference{Name: secretNamePrefix + secret1.Name},
					corev1.LocalObjectReference{Name: secretNamePrefix + secret2.Name},
				))
				Expect(managedResource.Labels).To(HaveKeyWithValue("origin", "gardener"))
				Expect(managedResource.Spec.InjectLabels).To(HaveKeyWithValue("shoot.gardener.cloud/no-cleanup", "true"))
			})

			It("should update existing secrets in shoot control plane namespace", func() {
				Expect(seedClient.Create(ctx, secret1)).To(Succeed())
				Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

				// Update the secret in garden namespace
				secret1.Data = map[string][]byte{"manifest": []byte("updated-data")}
				Expect(seedClient.Update(ctx, secret1)).To(Succeed())

				Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

				copiedSecret := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, copiedSecret)).To(Succeed())
				Expect(copiedSecret.Data).To(Equal(map[string][]byte{"manifest": []byte("updated-data")}))
			})

			Context("with shoot selector annotation", func() {
				It("should include secrets matching the shoot selector", func() {
					secret1.Annotations = map[string]string{
						"static-manifests.shoot.gardener.cloud/selector": `{"matchLabels":{"environment":"production"}}`,
					}
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())

					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(Succeed())
				})

				It("should exclude secrets not matching the shoot selector", func() {
					secret1.Annotations = map[string]string{
						"static-manifests.shoot.gardener.cloud/selector": `{"matchLabels":{"environment":"development"}}`,
					}
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())

					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(BeNotFoundError())
				})

				It("should include secrets with matchExpressions selector", func() {
					secret1.Annotations = map[string]string{
						"static-manifests.shoot.gardener.cloud/selector": `{"matchExpressions":[{"key":"environment","operator":"In","values":["production","staging"]}]}`,
					}
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())

					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(Succeed())
				})

				It("should include secrets without selector annotation", func() {
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())
					secret2.Annotations = map[string]string{
						"static-manifests.shoot.gardener.cloud/selector": `{"matchLabels":{"environment":"development"}}`,
					}
					Expect(seedClient.Create(ctx, secret2)).To(Succeed())

					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					// secret1 should be copied (no selector)
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(Succeed())

					// secret2 should not be copied (non-matching selector)
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret2.Name}, &corev1.Secret{})).To(BeNotFoundError())
				})

				It("should return error for invalid selector JSON", func() {
					secret1.Annotations = map[string]string{
						"static-manifests.shoot.gardener.cloud/selector": `{invalid json}`,
					}
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())

					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(MatchError(ContainSubstring("failed unmarshalling shoot selector")))
				})

				It("should return error for invalid label selector", func() {
					secret1.Annotations = map[string]string{
						"static-manifests.shoot.gardener.cloud/selector": `{"matchExpressions":[{"key":"","operator":"Invalid","values":["test"]}]}`,
					}
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())

					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(MatchError(ContainSubstring("failed parsing label selector")))
				})
			})

			Context("cleanup of old secrets", func() {
				It("should delete secrets from shoot namespace that are no longer in garden namespace", func() {
					// Create secrets in garden namespace
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())
					Expect(seedClient.Create(ctx, secret2)).To(Succeed())
					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					// Remove secret2 from garden namespace
					Expect(seedClient.Delete(ctx, secret2)).To(Succeed())
					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					// secret1 should still exist
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(Succeed())

					// secret2 should be deleted
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret2.Name}, &corev1.Secret{})).To(BeNotFoundError())
				})

				It("should not delete unrelated secrets in shoot namespace", func() {
					unrelatedSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "unrelated-secret",
							Namespace: controlPlaneNamespace,
							Labels:    map[string]string{"some-other-label": "true"},
						},
						Data: map[string][]byte{"data": []byte("test")},
					}
					Expect(seedClient.Create(ctx, unrelatedSecret)).To(Succeed())

					Expect(seedClient.Create(ctx, secret1)).To(Succeed())
					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					// Unrelated secret (without static manifests label) should still exist
					Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(unrelatedSecret), &corev1.Secret{})).To(Succeed())
				})

				It("should handle secrets that no longer match the selector", func() {
					secret1.Annotations = map[string]string{
						"static-manifests.shoot.gardener.cloud/selector": `{"matchLabels":{"environment":"production"}}`,
					}
					Expect(seedClient.Create(ctx, secret1)).To(Succeed())
					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					// Verify secret was copied
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(Succeed())

					// Update selector to not match
					secret1.Annotations["static-manifests.shoot.gardener.cloud/selector"] = `{"matchLabels":{"environment":"development"}}`
					Expect(seedClient.Update(ctx, secret1)).To(Succeed())
					Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

					// Secret should be deleted from shoot namespace
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(BeNotFoundError())
				})
			})

			It("should handle multiple secrets with mixed selectors", func() {
				secret1.Annotations = map[string]string{
					"static-manifests.shoot.gardener.cloud/selector": `{"matchLabels":{"environment":"production"}}`,
				}
				Expect(seedClient.Create(ctx, secret1)).To(Succeed())

				// secret2 has no selector
				Expect(seedClient.Create(ctx, secret2)).To(Succeed())

				secret3.Annotations = map[string]string{
					"static-manifests.shoot.gardener.cloud/selector": `{"matchLabels":{"environment":"development"}}`,
				}
				Expect(seedClient.Create(ctx, secret3)).To(Succeed())

				Expect(botanist.PopulateStaticManifestsFromSeedToShoot(ctx)).To(Succeed())

				// secret1 should be copied (matching selector)
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret1.Name}, &corev1.Secret{})).To(Succeed())

				// secret2 should be copied (no selector)
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret2.Name}, &corev1.Secret{})).To(Succeed())

				// secret3 should not be copied (non-matching selector)
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: secretNamePrefix + secret3.Name}, &corev1.Secret{})).To(BeNotFoundError())

				// Verify managed resource references only secret1 and secret2
				managedResource := &resourcesv1alpha1.ManagedResource{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: managedResourceName}, managedResource)).To(Succeed())
				Expect(managedResource.Spec.SecretRefs).To(HaveExactElements(
					corev1.LocalObjectReference{Name: secretNamePrefix + secret1.Name},
					corev1.LocalObjectReference{Name: secretNamePrefix + secret2.Name},
				))
			})
		})
	})
})
