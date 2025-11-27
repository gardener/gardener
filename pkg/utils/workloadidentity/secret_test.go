// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var _ = Describe("#Secret", func() {
	const (
		secretName      = "foo"
		secretNamespace = "namespace"
	)

	var (
		fakeClient client.Client
		ctx        context.Context
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		ctx = context.TODO()
	})

	It("should not be able to create a secret because workload identity info is not set", func() {
		secret, err := workloadidentity.NewSecret(secretName, secretNamespace)
		Expect(err).To(HaveOccurred())
		Expect(secret).To(BeNil())
		Expect(err.Error()).To(And(
			ContainSubstring("WorkloadIdentity name is not set"),
			ContainSubstring("WorkloadIdentity namespace is not set"),
			ContainSubstring("WorkloadIdentity provider type is not set"),
		))
	})

	It("should correctly create the secret", func() {
		secret, err := workloadidentity.NewSecret(
			secretName,
			secretNamespace,
			workloadidentity.For("wi-foo", "wi-ns", "provider"),
			workloadidentity.WithContextObject(securityv1alpha1.ContextObject{
				Kind:       "Shoot",
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Name:       "shoot-name",
				Namespace:  ptr.To("shoot-namespace"),
				UID:        "12345678-94af-4960-9774-0e9987654321",
			}),
			workloadidentity.WithProviderConfig(&runtime.RawExtension{
				Raw: []byte(`{"foo":"bar"}`),
			}),
			workloadidentity.WithLabels(map[string]string{
				"foo":                    "bar",
				"gardener.cloud/purpose": "cloudprovider",
			}),
			workloadidentity.WithAnnotations(map[string]string{"foo": "bar"}),
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(secret.Reconcile(ctx, fakeClient)).To(Succeed())
		got := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		}

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(got), got)).To(Succeed())
		Expect(got).To(Equal(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Annotations: map[string]string{
					"foo": "bar",
					"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"shoot-name","namespace":"shoot-namespace","uid":"12345678-94af-4960-9774-0e9987654321"}`,
					"workloadidentity.security.gardener.cloud/name":           "wi-foo",
					"workloadidentity.security.gardener.cloud/namespace":      "wi-ns",
				},
				Labels: map[string]string{
					"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
					"workloadidentity.security.gardener.cloud/provider": "provider",
					"foo":                    "bar",
					"gardener.cloud/purpose": "cloudprovider",
				},
				ResourceVersion: "1",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"config": []byte(`{"foo":"bar"}`),
			},
		}))
	})

	It("should correctly patch an existing secret", func() {
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Annotations: map[string]string{
					"foo": "bar",
					"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"shoot-name","namespace":"shoot-namespace","uid":"12345678-94af-4960-9774-0e9987654321"}`,
					"workloadidentity.security.gardener.cloud/name":           "wi-foo",
					"workloadidentity.security.gardener.cloud/namespace":      "wi-ns",
				},
				Labels: map[string]string{
					"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
					"workloadidentity.security.gardener.cloud/provider": "provider",
					"foo":                    "bar",
					"gardener.cloud/purpose": "cloudprovider",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"config": []byte(`{"foo":"bar"}`),
				"token":  []byte("token"),
				"foo":    []byte("bar"), // should be removed after the reconciliation of the secret
			},
		}

		Expect(fakeClient.Create(ctx, existing)).To(Succeed())

		secret, err := workloadidentity.NewSecret(
			secretName,
			secretNamespace,
			workloadidentity.For("new-name", "new-namespace", "new-provider"),
			workloadidentity.WithLabels(map[string]string{"new-foo": "new-bar"}),
			workloadidentity.WithAnnotations(map[string]string{"new-foo": "new-bar"}),
			workloadidentity.WithProviderConfig(&runtime.RawExtension{
				Raw: []byte(`{"foo":"bar"}`),
			}),
			workloadidentity.WithContextObject(securityv1alpha1.ContextObject{
				Kind:       "Shoot",
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Name:       "new-name",
				Namespace:  ptr.To("new-namespace"),
				UID:        "12345678-94af-4960-9774-0e9987654321",
			}),
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(secret.Reconcile(ctx, fakeClient)).To(Succeed())
		got := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(got), got)).To(Succeed())
		Expect(got).To(Equal(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Annotations: map[string]string{
					"new-foo": "new-bar",
					"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"new-name","namespace":"new-namespace","uid":"12345678-94af-4960-9774-0e9987654321"}`,
					"workloadidentity.security.gardener.cloud/name":           "new-name",
					"workloadidentity.security.gardener.cloud/namespace":      "new-namespace",
				},
				Labels: map[string]string{
					"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
					"workloadidentity.security.gardener.cloud/provider": "new-provider",
					"new-foo": "new-bar",
				},
				ResourceVersion: "2",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"config": []byte(`{"foo":"bar"}`),
				"token":  []byte("token"),
			},
		}))
	})

	It("should remove unneeded info from an existing secret", func() {
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Annotations: map[string]string{
					"foo": "bar",
					"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"shoot-name","namespace":"shoot-namespace","uid":"12345678-94af-4960-9774-0e9987654321"}`,
					"workloadidentity.security.gardener.cloud/name":           "wi-foo",
					"workloadidentity.security.gardener.cloud/namespace":      "wi-ns",
				},
				Labels: map[string]string{
					"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
					"workloadidentity.security.gardener.cloud/provider": "provider",
					"foo":                    "bar",
					"gardener.cloud/purpose": "cloudprovider",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"config": []byte(`{"foo":"bar"}`),
				"token":  []byte("token"),
			},
		}

		Expect(fakeClient.Create(ctx, existing)).To(Succeed())

		secret, err := workloadidentity.NewSecret(
			secretName,
			secretNamespace,
			workloadidentity.For("new-name", "new-namespace", "new-provider"),
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(secret.Reconcile(ctx, fakeClient)).To(Succeed())
		got := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(got), got)).To(Succeed())
		Expect(got).To(Equal(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Annotations: map[string]string{
					"workloadidentity.security.gardener.cloud/name":      "new-name",
					"workloadidentity.security.gardener.cloud/namespace": "new-namespace",
				},
				Labels: map[string]string{
					"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
					"workloadidentity.security.gardener.cloud/provider": "new-provider",
				},
				ResourceVersion: "2",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"token": []byte("token"),
			},
		}))
	})

	It("should not set the `config` data key when provider config is nil", func() {
		secret, err := workloadidentity.NewSecret(secretName, secretNamespace,
			workloadidentity.For("wi-foo", "wi-ns", "provider"),
			workloadidentity.WithProviderConfig(nil),
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(secret.Reconcile(ctx, fakeClient)).To(Succeed())
		got := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		}

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(got), got)).To(Succeed())
		Expect(got.Data).ToNot(HaveKey("config"))
	})

	It("should compare existing secret with the generated one", func() {
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Annotations: map[string]string{
					"foo": "bar",
					"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"shoot-name","namespace":"shoot-namespace","uid":"12345678-94af-4960-9774-0e9987654321"}`,
					"workloadidentity.security.gardener.cloud/name":           "wi-foo",
					"workloadidentity.security.gardener.cloud/namespace":      "wi-ns",
				},
				Labels: map[string]string{
					"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
					"workloadidentity.security.gardener.cloud/provider": "provider",
					"foo": "bar",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"config": []byte(`{"foo":"bar"}`),
				"token":  []byte("token"),
				"foo":    []byte("bar"),
			},
		}

		secret, err := workloadidentity.NewSecret(
			secretName,
			secretNamespace,
			workloadidentity.For("wi-foo", "wi-ns", "provider"),
			workloadidentity.WithLabels(map[string]string{"foo": "bar"}),
			workloadidentity.WithAnnotations(map[string]string{"foo": "bar"}),
			workloadidentity.WithProviderConfig(&runtime.RawExtension{
				Raw: []byte(`{"foo":"bar"}`),
			}),
			workloadidentity.WithContextObject(securityv1alpha1.ContextObject{
				Kind:       "Shoot",
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Name:       "shoot-name",
				Namespace:  ptr.To("shoot-namespace"),
				UID:        "12345678-94af-4960-9774-0e9987654321",
			}),
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(secret.Equal(existing)).To(BeTrue())

		By("Adding data fields should keep equality")
		existing.Data["equal"] = []byte("value")
		Expect(secret.Equal(existing)).To(BeTrue())

		By("deleting workloadidentity relevant annotation should make secrets differ")
		delete(existing.Annotations, "workloadidentity.security.gardener.cloud/name")
		Expect(secret.Equal(existing)).To(BeFalse())
	})

	var _ = Describe("#Deploy", func() {
		const (
			secretName      = "target-secret"
			secretNamespace = "target-namespace"
			wiName          = "workload-identity"
			wiNamespace     = "wi-namespace"
			providerType    = "test-provider"
		)

		var (
			fakeClient       client.Client
			ctx              context.Context
			workloadIdentity *securityv1alpha1.WorkloadIdentity
			referringObj     *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			ctx = context.Background()

			workloadIdentity = &securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      wiName,
					Namespace: wiNamespace,
				},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"aud1", "aud2"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: providerType,
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"key":"value"}`),
						},
					},
				},
			}

			referringObj = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: "garden-test",
					UID:       "shoot-uid-12345",
				},
			}
		})

		It("should successfully deploy a new secret", func() {
			err := workloadidentity.Deploy(ctx, fakeClient, workloadIdentity, secretName, secretNamespace, nil, nil, referringObj)
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)).To(Succeed())

			Expect(secret.Name).To(Equal(secretName))
			Expect(secret.Namespace).To(Equal(secretNamespace))
			Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))

			// Verify annotations
			Expect(secret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/name", wiName))
			Expect(secret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/namespace", wiNamespace))
			Expect(secret.Annotations).To(HaveKey("workloadidentity.security.gardener.cloud/context-object"))

			// Verify labels
			Expect(secret.Labels).To(HaveKeyWithValue("security.gardener.cloud/purpose", "workload-identity-token-requestor"))
			Expect(secret.Labels).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/provider", providerType))

			// Verify data
			Expect(secret.Data).To(HaveKey("config"))
		})

		It("should deploy secret with custom annotations and labels", func() {
			customAnnotations := map[string]string{
				"custom-annotation": "custom-value",
				"foo":               "bar",
			}
			customLabels := map[string]string{
				"custom-label": "custom-value",
				"env":          "production",
			}

			err := workloadidentity.Deploy(ctx, fakeClient, workloadIdentity, secretName, secretNamespace, customAnnotations, customLabels, referringObj)
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)).To(Succeed())

			// Verify custom annotations are present along with workload identity annotations
			Expect(secret.Annotations).To(HaveKeyWithValue("custom-annotation", "custom-value"))
			Expect(secret.Annotations).To(HaveKeyWithValue("foo", "bar"))
			Expect(secret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/name", wiName))

			// Verify custom labels are present along with workload identity labels
			Expect(secret.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
			Expect(secret.Labels).To(HaveKeyWithValue("env", "production"))
			Expect(secret.Labels).To(HaveKeyWithValue("security.gardener.cloud/purpose", "workload-identity-token-requestor"))
		})

		It("should update an existing secret", func() {
			// Create an initial secret
			existing := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: secretNamespace,
					Annotations: map[string]string{
						"old-annotation": "old-value",
					},
					Labels: map[string]string{
						"old-label": "old-value",
					},
				},
				Data: map[string][]byte{
					"old-key": []byte("old-data"),
					"token":   []byte("existing-token"),
				},
			}
			Expect(fakeClient.Create(ctx, existing)).To(Succeed())

			// Deploy with new configuration
			err := workloadidentity.Deploy(ctx, fakeClient, workloadIdentity, secretName, secretNamespace, nil, nil, referringObj)
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)).To(Succeed())

			// Verify old annotations are replaced
			Expect(secret.Annotations).ToNot(HaveKey("old-annotation"))
			Expect(secret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/name", wiName))

			// Verify old labels are replaced
			Expect(secret.Labels).ToNot(HaveKey("old-label"))
			Expect(secret.Labels).To(HaveKeyWithValue("security.gardener.cloud/purpose", "workload-identity-token-requestor"))

			// Verify token is preserved but old data key is removed
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte("existing-token")))
			Expect(secret.Data).ToNot(HaveKey("old-key"))
			Expect(secret.Data).To(HaveKey("config"))
		})

		It("should handle workload identity without provider config", func() {
			workloadIdentity.Spec.TargetSystem.ProviderConfig = nil

			err := workloadidentity.Deploy(ctx, fakeClient, workloadIdentity, secretName, secretNamespace, nil, nil, referringObj)
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)).To(Succeed())

			// Verify config key is not present when provider config is nil
			Expect(secret.Data).ToNot(HaveKey("config"))
		})

		It("should include context object with namespace for namespaced referring object", func() {
			err := workloadidentity.Deploy(ctx, fakeClient, workloadIdentity, secretName, secretNamespace, nil, nil, referringObj)
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)).To(Succeed())

			contextObjJSON := secret.Annotations["workloadidentity.security.gardener.cloud/context-object"]
			Expect(contextObjJSON).ToNot(BeEmpty())

			var contextObj securityv1alpha1.ContextObject
			Expect(json.Unmarshal([]byte(contextObjJSON), &contextObj)).To(Succeed())

			Expect(contextObj.Kind).To(Equal("Shoot"))
			Expect(contextObj.APIVersion).To(Equal(gardencorev1beta1.SchemeGroupVersion.String()))
			Expect(contextObj.Name).To(Equal("test-shoot"))
			Expect(contextObj.Namespace).To(Equal(ptr.To("garden-test")))
			Expect(contextObj.UID).To(Equal(referringObj.UID))
		})

		It("should include context object without namespace for cluster-scoped referring object", func() {
			clusterScopedObj := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-seed",
					UID:  "seed-uid-67890",
				},
			}

			err := workloadidentity.Deploy(ctx, fakeClient, workloadIdentity, secretName, secretNamespace, nil, nil, clusterScopedObj)
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)).To(Succeed())

			contextObjJSON := secret.Annotations["workloadidentity.security.gardener.cloud/context-object"]
			Expect(contextObjJSON).ToNot(BeEmpty())

			var contextObj securityv1alpha1.ContextObject
			Expect(json.Unmarshal([]byte(contextObjJSON), &contextObj)).To(Succeed())

			Expect(contextObj.Kind).To(Equal("Seed"))
			Expect(contextObj.APIVersion).To(Equal(gardencorev1beta1.SchemeGroupVersion.String()))
			Expect(contextObj.Name).To(Equal("test-seed"))
			Expect(contextObj.Namespace).To(BeNil())
			Expect(contextObj.UID).To(Equal(clusterScopedObj.UID))
		})

	})
})
