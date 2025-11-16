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
						"security.gardener.cloud/purpose":                     "workload-identity-token-requestor",
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
					"security.gardener.cloud/purpose":                     "workload-identity-token-requestor",
				}),
			)).To(Succeed())
			Expect(workloadIdentitySecrets.Items).To(BeEmpty())
		})
	})
})
