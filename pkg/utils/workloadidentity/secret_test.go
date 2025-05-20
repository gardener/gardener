// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"context"

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
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var _ = Describe("#Secret", func() {
	var (
		fakeClient      client.Client
		secretName      = "foo"
		secretNamespace = "namespace"
		ctx             context.Context
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
})
