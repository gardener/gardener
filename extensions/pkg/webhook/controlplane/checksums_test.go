// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Checksums", func() {
	const (
		namespace  = "shoot--foo--bar"
		secretName = "my-secret"
		cmName     = "my-configmap"
	)

	var (
		ctx        context.Context
		fakeClient client.Client
		template   *corev1.PodTemplateSpec
	)

	BeforeEach(func() {
		ctx = context.Background()
		template = &corev1.PodTemplateSpec{}
	})

	Describe("#EnsureSecretChecksumAnnotation", func() {
		It("should add the correct checksum annotation for the secret", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      secretName,
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(secret).Build()

			Expect(EnsureSecretChecksumAnnotation(ctx, template, fakeClient, namespace, secretName)).To(Succeed())
			Expect(template.Annotations).To(HaveKeyWithValue(
				"checksum/secret-"+secretName,
				utils.ComputeChecksum(secret.Data),
			))
		})

		It("should add the annotation even if the template already has other annotations", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      secretName,
				},
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(secret).Build()

			template.Annotations = map[string]string{
				"existing-annotation": "existing-value",
			}

			Expect(EnsureSecretChecksumAnnotation(ctx, template, fakeClient, namespace, secretName)).To(Succeed())
			Expect(template.Annotations).To(HaveKeyWithValue("existing-annotation", "existing-value"))
			Expect(template.Annotations).To(HaveKeyWithValue(
				"checksum/secret-"+secretName,
				utils.ComputeChecksum(secret.Data),
			))
		})
	})

	Describe("#EnsureConfigMapChecksumAnnotation", func() {
		It("should add the correct checksum annotation for the configmap", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      cmName,
				},
				Data: map[string]string{
					"config.yaml": "foo: bar",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(cm).Build()

			Expect(EnsureConfigMapChecksumAnnotation(ctx, template, fakeClient, namespace, cmName)).To(Succeed())
			Expect(template.Annotations).To(HaveKeyWithValue(
				"checksum/configmap-"+cmName,
				utils.ComputeChecksum(cm.Data),
			))
		})

		It("should add the annotation even if the template already has other annotations", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      cmName,
				},
				Data: map[string]string{
					"key": "value",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(cm).Build()

			template.Annotations = map[string]string{
				"pre-existing": "annotation",
			}

			Expect(EnsureConfigMapChecksumAnnotation(ctx, template, fakeClient, namespace, cmName)).To(Succeed())
			Expect(template.Annotations).To(HaveKeyWithValue("pre-existing", "annotation"))
			Expect(template.Annotations).To(HaveKeyWithValue(
				"checksum/configmap-"+cmName,
				utils.ComputeChecksum(cm.Data),
			))
		})

		It("should compute different checksums for different configmap data", func() {
			cm1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "cm-one",
				},
				Data: map[string]string{"key": "value1"},
			}
			cm2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "cm-two",
				},
				Data: map[string]string{"key": "value2"},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(cm1, cm2).Build()

			Expect(EnsureConfigMapChecksumAnnotation(ctx, template, fakeClient, namespace, "cm-one")).To(Succeed())
			checksum1 := template.Annotations["checksum/configmap-cm-one"]

			template2 := &corev1.PodTemplateSpec{}
			Expect(EnsureConfigMapChecksumAnnotation(ctx, template2, fakeClient, namespace, "cm-two")).To(Succeed())
			checksum2 := template2.Annotations["checksum/configmap-cm-two"]

			Expect(checksum1).NotTo(Equal(checksum2))
		})
	})
})
