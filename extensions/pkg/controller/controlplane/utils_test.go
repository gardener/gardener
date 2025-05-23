// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/extensions/pkg/controller/controlplane"
)

var _ = Describe("Utils", func() {
	Describe("#MergeSecretMaps", func() {
		var (
			test0 = getSecret("test0", "default", nil)
			test1 = getSecret("test1", "default", nil)
			a     = map[string]*corev1.Secret{
				"test0": test0,
			}
			b = map[string]*corev1.Secret{
				"test1": test1,
			}
		)

		It("should return an empty map if both given maps are empty", func() {
			Expect(MergeSecretMaps(nil, nil)).To(BeEmpty())
		})
		It("should return the other map of one of the given maps is empty", func() {
			Expect(MergeSecretMaps(a, nil)).To(Equal(a))
		})
		It("should properly merge the given non-empty maps", func() {
			result := MergeSecretMaps(a, b)
			Expect(result).To(HaveKeyWithValue("test0", test0))
			Expect(result).To(HaveKeyWithValue("test1", test1))
		})
	})

	Describe("#ComputeChecksums", func() {
		var (
			secrets = map[string]*corev1.Secret{
				"test-secret": getSecret("test-secret", "default", map[string][]byte{"foo": []byte("bar")}),
			}
			cms = map[string]*corev1.ConfigMap{
				"test-config": getConfigMap("test-config", "default", map[string]string{"abc": "xyz"}),
			}
		)
		It("should compute all checksums for the given secrets and configmpas", func() {
			checksums := ComputeChecksums(secrets, cms)
			Expect(checksums).To(HaveKeyWithValue("test-secret", "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432"))
			Expect(checksums).To(HaveKeyWithValue("test-config", "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606"))
		})
	})
})

func getSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func getConfigMap(name, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}
