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
		It("should return the first map if the second is nil", func() {
			result := MergeSecretMaps(nil, b)
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("test1", test1))
		})
		It("should properly merge the given non-empty maps", func() {
			result := MergeSecretMaps(a, b)
			Expect(result).To(HaveKeyWithValue("test0", test0))
			Expect(result).To(HaveKeyWithValue("test1", test1))
		})
		It("should let values from map b override values from map a for the same key", func() {
			overrideSecret := getSecret("override", "other-ns", map[string][]byte{"key": []byte("val")})
			mapA := map[string]*corev1.Secret{
				"shared-key": test0,
			}
			mapB := map[string]*corev1.Secret{
				"shared-key": overrideSecret,
			}

			result := MergeSecretMaps(mapA, mapB)
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("shared-key", overrideSecret))
		})
		It("should not modify the original maps", func() {
			mapA := map[string]*corev1.Secret{
				"a": test0,
			}
			mapB := map[string]*corev1.Secret{
				"b": test1,
			}

			result := MergeSecretMaps(mapA, mapB)
			Expect(result).To(HaveLen(2))
			Expect(mapA).To(HaveLen(1))
			Expect(mapB).To(HaveLen(1))
		})
		It("should return an empty (but non-nil) map when merging two empty maps", func() {
			result := MergeSecretMaps(map[string]*corev1.Secret{}, map[string]*corev1.Secret{})
			Expect(result).NotTo(BeNil())
			Expect(result).To(BeEmpty())
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

		It("should return an empty map when both inputs are nil", func() {
			checksums := ComputeChecksums(nil, nil)
			Expect(checksums).NotTo(BeNil())
			Expect(checksums).To(BeEmpty())
		})

		It("should return an empty map when both inputs are empty", func() {
			checksums := ComputeChecksums(map[string]*corev1.Secret{}, map[string]*corev1.ConfigMap{})
			Expect(checksums).NotTo(BeNil())
			Expect(checksums).To(BeEmpty())
		})

		It("should compute checksums for only secrets when configmaps is nil", func() {
			checksums := ComputeChecksums(secrets, nil)
			Expect(checksums).To(HaveLen(1))
			Expect(checksums).To(HaveKey("test-secret"))
		})

		It("should compute checksums for only configmaps when secrets is nil", func() {
			checksums := ComputeChecksums(nil, cms)
			Expect(checksums).To(HaveLen(1))
			Expect(checksums).To(HaveKey("test-config"))
		})

		It("should compute different checksums for secrets with different data", func() {
			secret1 := map[string]*corev1.Secret{
				"s1": getSecret("s1", "default", map[string][]byte{"key": []byte("value1")}),
			}
			secret2 := map[string]*corev1.Secret{
				"s2": getSecret("s2", "default", map[string][]byte{"key": []byte("value2")}),
			}
			checksums1 := ComputeChecksums(secret1, nil)
			checksums2 := ComputeChecksums(secret2, nil)
			Expect(checksums1["s1"]).NotTo(Equal(checksums2["s2"]))
		})

		It("should compute the same checksum for secrets with the same data", func() {
			secret1 := map[string]*corev1.Secret{
				"s1": getSecret("s1", "ns1", map[string][]byte{"key": []byte("value")}),
			}
			secret2 := map[string]*corev1.Secret{
				"s2": getSecret("s2", "ns2", map[string][]byte{"key": []byte("value")}),
			}
			checksums1 := ComputeChecksums(secret1, nil)
			checksums2 := ComputeChecksums(secret2, nil)
			Expect(checksums1["s1"]).To(Equal(checksums2["s2"]))
		})

		It("should handle multiple secrets and configmaps", func() {
			multiSecrets := map[string]*corev1.Secret{
				"secret-a": getSecret("secret-a", "default", map[string][]byte{"a": []byte("1")}),
				"secret-b": getSecret("secret-b", "default", map[string][]byte{"b": []byte("2")}),
			}
			multiCMs := map[string]*corev1.ConfigMap{
				"cm-a": getConfigMap("cm-a", "default", map[string]string{"x": "y"}),
				"cm-b": getConfigMap("cm-b", "default", map[string]string{"z": "w"}),
			}
			checksums := ComputeChecksums(multiSecrets, multiCMs)
			Expect(checksums).To(HaveLen(4))
			Expect(checksums).To(HaveKey("secret-a"))
			Expect(checksums).To(HaveKey("secret-b"))
			Expect(checksums).To(HaveKey("cm-a"))
			Expect(checksums).To(HaveKey("cm-b"))
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
