// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubeproxyconfigv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	. "github.com/gardener/gardener/pkg/provider-local/webhook/seed"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const dataKeyConfig = "config.yaml"

var _ = Describe("Mutator", func() {
	var (
		ctx     = context.Background()
		mutator = NewMutator()
		codec   = kubeproxy.NewConfigCodec()
		decoder = serializer.NewCodecFactory(kubernetes.ShootScheme).UniversalDeserializer()
	)

	// newConfigYAML returns a serialized kube-proxy configuration with the given conntrack maxPerCore.
	newConfigYAML := func(maxPerCore int32) string {
		data, err := codec.Encode(&kubeproxyconfigv1alpha1.KubeProxyConfiguration{
			Conntrack: kubeproxyconfigv1alpha1.KubeProxyConntrackConfiguration{
				MaxPerCore: new(maxPerCore),
			},
		})
		Expect(err).NotTo(HaveOccurred())
		return data
	}

	// newSecret builds a kube-proxy ManagedResource secret carrying the given objects.
	newSecret := func(objects ...client.Object) *corev1.Secret {
		data, err := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).AddAllAndSerialize(objects...)
		Expect(err).NotTo(HaveOccurred())

		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-shoot-core-kube-proxy",
				Namespace: "shoot--foo--bar",
				Labels:    map[string]string{"component": "kube-proxy"},
			},
			Data: data,
		}
	}

	It("should set conntrack maxPerCore to 0 in the kube-proxy config ConfigMap and leave other objects untouched", func() {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeproxy.ConfigNamePrefix + "-abc123",
				Namespace: "kube-system",
			},
			Data: map[string]string{dataKeyConfig: newConfigYAML(524288)},
		}
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy",
				Namespace: "kube-system",
			},
		}

		secret := newSecret(configMap, serviceAccount)

		Expect(mutator.Mutate(ctx, secret, nil)).To(Succeed())

		objects, err := managedresources.ExtractObjectsFromSecret(decoder, secret)
		Expect(err).NotTo(HaveOccurred())

		var (
			foundConfigMap      bool
			foundServiceAccount bool
		)
		for _, obj := range objects {
			switch o := obj.(type) {
			case *corev1.ConfigMap:
				foundConfigMap = true
				config, err := codec.Decode(o.Data[dataKeyConfig])
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Conntrack.MaxPerCore).To(HaveValue(Equal(int32(0))))
			case *corev1.ServiceAccount:
				foundServiceAccount = true
				Expect(o.Name).To(Equal("kube-proxy"))
			}
		}
		Expect(foundConfigMap).To(BeTrue())
		Expect(foundServiceAccount).To(BeTrue())
	})

	It("should not modify a secret that does not contain the kube-proxy config ConfigMap", func() {
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy",
				Namespace: "kube-system",
			},
		}

		secret := newSecret(serviceAccount)
		originalData := make(map[string][]byte, len(secret.Data))
		for k, v := range secret.Data {
			originalData[k] = append([]byte(nil), v...)
		}

		Expect(mutator.Mutate(ctx, secret, nil)).To(Succeed())
		Expect(secret.Data).To(Equal(originalData))
	})

	It("should not mutate an object that is being deleted", func() {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeproxy.ConfigNamePrefix + "-abc123",
				Namespace: "kube-system",
			},
			Data: map[string]string{dataKeyConfig: newConfigYAML(524288)},
		}
		secret := newSecret(configMap)
		now := metav1.Now()
		secret.DeletionTimestamp = &now

		originalData := make(map[string][]byte, len(secret.Data))
		for k, v := range secret.Data {
			originalData[k] = append([]byte(nil), v...)
		}

		Expect(mutator.Mutate(ctx, secret, nil)).To(Succeed())
		Expect(secret.Data).To(Equal(originalData))
	})
})
