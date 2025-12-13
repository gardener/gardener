// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"encoding/base64"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Resources", func() {
	var (
		ctx              context.Context
		fakeGardenClient client.Client
		resources        []gardencorev1beta1.NamedResourceReference
		sourceNamespace  string
		targetNamespace  string

		annotations map[string]string
		labels      map[string]string

		secret           *corev1.Secret
		configMap        *corev1.ConfigMap
		workloadIdentity *securityv1alpha1.WorkloadIdentity
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeGardenClient = fake.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		sourceNamespace = "garden"
		targetNamespace = "garden--project--shoot"

		annotations = map[string]string{
			"annotationKey": "annotationValue",
			"kubectl.kubernetes.io/last-applied-configuration": "last-applied-configuration",
		}

		labels = map[string]string{
			"labelKey": "labelValue",
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-secret",
				Namespace:   sourceNamespace,
				Annotations: annotations,
				Labels:      labels,
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}
		Expect(fakeGardenClient.Create(ctx, secret)).To(Succeed())

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-configmap",
				Namespace:   sourceNamespace,
				Finalizers:  []string{"finalizer"},
				Annotations: annotations,
				Labels:      labels,
			},
			Data: map[string]string{
				"key": "value",
			},
		}
		Expect(fakeGardenClient.Create(ctx, configMap)).To(Succeed())

		workloadIdentity = &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-workload-identity",
				Namespace:   sourceNamespace,
				Finalizers:  []string{"finalizer"},
				Annotations: annotations,
				Labels:      labels,
			},
			Spec: securityv1alpha1.WorkloadIdentitySpec{
				TargetSystem: securityv1alpha1.TargetSystem{
					Type:           "test",
					ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
				},
			},
		}
		Expect(fakeGardenClient.Create(ctx, workloadIdentity)).To(Succeed())

		resources = []gardencorev1beta1.NamedResourceReference{
			{
				Name: "referenced-test-secret",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secret.Name,
				},
			},
			{
				Name: "referenced-test-configmap",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       configMap.Name,
				},
			},
			{
				Name: "referenced-test-workload-identity",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Name:       workloadIdentity.Name,
				},
			},
		}
	})

	Describe("#PrepareReferencedResourcesForSeedCopy", func() {
		It("should prepare unstructured objects correctly", func() {
			unstructuredObjs, err := PrepareReferencedResourcesForSeedCopy(ctx, fakeGardenClient, resources, sourceNamespace, targetNamespace)

			Expect(unstructuredObjs).To(HaveLen(2))
			Expect(err).NotTo(HaveOccurred())

			for _, unstructuredObj := range unstructuredObjs {
				Expect(unstructuredObj.GetNamespace()).To(Equal(targetNamespace), unstructuredObj.GetName()+" should have target namespace "+targetNamespace)
				Expect(unstructuredObj.GetAnnotations()).To(BeEmpty(), unstructuredObj.GetName()+" should have no annotations")
				Expect(unstructuredObj.GetLabels()).To(BeEmpty(), unstructuredObj.GetName()+" should have no labels")
				Expect(unstructuredObj.GetFinalizers()).To(BeEmpty(), unstructuredObj.GetName()+" should have no finalizers")
				Expect(unstructuredObj.Object).To(HaveKey("data"), unstructuredObj.GetName()+" should have data field")

				switch unstructuredObj.GetKind() {
				case "Secret":
					Expect(unstructuredObj.GetName()).To(Equal("ref-" + secret.Name))
					Expect(unstructuredObj.Object["data"]).To(WithTransform(func(data interface{}) (map[string][]byte, error) {
						byteMap := make(map[string][]byte)
						for k, v := range data.(map[string]interface{}) {
							byteMap[k], err = base64.StdEncoding.DecodeString(v.(string))
							if err != nil {
								return nil, err
							}
						}
						return byteMap, nil
					}, Equal(secret.Data)))
				case "ConfigMap":
					Expect(unstructuredObj.GetName()).To(Equal("ref-" + configMap.Name))
					Expect(unstructuredObj.Object["data"]).To(WithTransform(func(data interface{}) map[string]string {
						strMap := make(map[string]string)
						for k, v := range data.(map[string]interface{}) {
							strMap[k] = v.(string)
						}
						return strMap
					}, Equal(configMap.Data)))
				default:
					Fail(unstructuredObj.GetName() + " has unexpected kind")
				}
			}
		})

		It("should return an error if the referenced object is not found", func() {
			resources[0].ResourceRef.Name = "non-existing-secret"

			unstructuredObjs, err := PrepareReferencedResourcesForSeedCopy(ctx, fakeGardenClient, resources, sourceNamespace, targetNamespace)
			Expect(unstructuredObjs).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("object not found")))
		})
	})

	Describe("#ReconcileWorkloadIdentityReferencedResources", func() {
		var (
			fakeSeedClient client.Client
			shoot          *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			fakeSeedClient = fake.NewClientBuilder().Build()
			shoot = &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: sourceNamespace,
					UID:       "9b60abb0-62c9-43b9-9a63-63907e696466",
				},
			}
		})

		It("should create and clean up workload identity secrets correctly", func() {
			Expect(ReconcileWorkloadIdentityReferencedResources(ctx, fakeGardenClient, fakeSeedClient, resources, sourceNamespace, targetNamespace, shoot)).To(Succeed())

			secrets := &corev1.SecretList{}
			Expect(fakeSeedClient.List(ctx, secrets,
				client.InNamespace(targetNamespace),
				client.MatchingLabels(map[string]string{"workloadidentity.security.gardener.cloud/referenced": "true"}),
			)).To(Succeed())

			Expect(secrets.Items).To(HaveLen(1))
			secret := &secrets.Items[0]
			Expect(secret.Namespace).To(Equal(targetNamespace))
			Expect(secret.Name).To(Equal("workload-identity-ref-" + workloadIdentity.Name))
			Expect(secret.Labels).To(And(
				HaveKeyWithValue("workloadidentity.security.gardener.cloud/referenced", "true"),
				HaveKeyWithValue("security.gardener.cloud/purpose", "workload-identity-token-requestor"),
				HaveKeyWithValue("workloadidentity.security.gardener.cloud/provider", "test"),
			))
			Expect(secret.Annotations).To(And(
				HaveKeyWithValue("workloadidentity.security.gardener.cloud/namespace", sourceNamespace),
				HaveKeyWithValue("workloadidentity.security.gardener.cloud/name", workloadIdentity.Name),
				HaveKeyWithValue("workloadidentity.security.gardener.cloud/context-object", `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"test-shoot","namespace":"garden","uid":"9b60abb0-62c9-43b9-9a63-63907e696466"}`),
			))
			Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(secret.Data).To(HaveKeyWithValue("config", []byte(`{"key":"value"}`)))

			noWorkloadIdentityResources := slices.DeleteFunc(resources, func(r gardencorev1beta1.NamedResourceReference) bool { return r.ResourceRef.Kind == "WorkloadIdentity" })
			Expect(ReconcileWorkloadIdentityReferencedResources(ctx, fakeGardenClient, fakeSeedClient, noWorkloadIdentityResources, sourceNamespace, targetNamespace, shoot)).To(Succeed())
			err := fakeSeedClient.Get(ctx, client.ObjectKey{Namespace: targetNamespace, Name: secret.Name}, secret)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
		})

		Describe("#DestroyWorkloadIdentityReferencedResources", func() {
			It("should destroy workload identity secrets correctly", func() {
				workloadIdentitySecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "workload-identity-ref-foo",
						Namespace: targetNamespace,
						Labels:    map[string]string{"workloadidentity.security.gardener.cloud/referenced": "true"},
					},
				}
				unrelatedSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unrelated-secret",
						Namespace: targetNamespace,
					},
				}

				Expect(fakeSeedClient.Create(ctx, workloadIdentitySecret)).To(Succeed())
				Expect(fakeSeedClient.Create(ctx, unrelatedSecret)).To(Succeed())
				Expect(DestroyWorkloadIdentityReferencedResources(ctx, fakeSeedClient, targetNamespace)).To(Succeed())

				secrets := &corev1.SecretList{}
				Expect(fakeSeedClient.List(ctx, secrets,
					client.InNamespace(targetNamespace),
					client.MatchingLabels(map[string]string{"workloadidentity.security.gardener.cloud/referenced": "true"}),
				)).To(Succeed())
				Expect(secrets.Items).To(BeEmpty())
				By("ensuring that unrelated secret still exists")
				Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(unrelatedSecret), unrelatedSecret)).To(Succeed())
			})
		})
	})
})
