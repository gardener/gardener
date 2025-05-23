// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresources_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"

	. "github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Registry", func() {
	var (
		scheme       = runtime.NewScheme()
		serial       = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
		codecFactory = serializer.NewCodecFactory(scheme)

		registry *Registry

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret:name",
				Namespace: "foo",
				Annotations: map[string]string{
					"foo.bar/test-939fc8c2": "3",
					"foo.bar/test-ea8edc28": "7",
					"foo.bar/test-9dca243c": "1",
					"foo.bar/test-47fc132b": "2",
					"foo.bar/test-2871f8c4": "4",
					"foo.bar/test-07679f5e": "5",
					"foo.bar/test-d2718f1d": "6",
				},
				Labels: map[string]string{
					"foo.bar/test-939fc8c2": "3",
					"foo.bar/test-47fc132b": "2",
					"foo.bar/test-d2718f1d": "6",
					"foo.bar/test-9dca243c": "1",
					"foo.bar/test-07679f5e": "5",
					"foo.bar/test-2871f8c4": "4",
					"foo.bar/test-ea8edc28": "7",
				},
			},
		}
		secretSerialized = `apiVersion: v1
kind: Secret
metadata:
  annotations:
    foo.bar/test-9dca243c: "1"
    foo.bar/test-47fc132b: "2"
    foo.bar/test-939fc8c2: "3"
    foo.bar/test-2871f8c4: "4"
    foo.bar/test-07679f5e: "5"
    foo.bar/test-d2718f1d: "6"
    foo.bar/test-ea8edc28: "7"
  creationTimestamp: null
  labels:
    foo.bar/test-9dca243c: "1"
    foo.bar/test-47fc132b: "2"
    foo.bar/test-939fc8c2: "3"
    foo.bar/test-2871f8c4: "4"
    foo.bar/test-07679f5e: "5"
    foo.bar/test-d2718f1d: "6"
    foo.bar/test-ea8edc28: "7"
  name: ` + secret.Name + `
  namespace: ` + secret.Namespace + `
`

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rolebinding.name",
				Namespace: "bar",
			},
		}
		roleBindingSerialized = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: ` + roleBinding.Name + `
  namespace: ` + roleBinding.Namespace + `
roleRef:
  apiGroup: ""
  kind: ""
  name: ""
`
	)

	BeforeEach(func() {
		Expect(kubernetesscheme.AddToScheme(scheme)).To(Succeed())

		registry = NewRegistry(scheme, codecFactory, serial)
	})

	Describe("#Add", func() {
		It("should successfully add the objects", func() {
			Expect(registry.Add(&corev1.Secret{}, &corev1.ConfigMap{})).To(Succeed())
		})

		It("should do nothing because the object is nil", func() {
			Expect(registry.Add(nil)).To(Succeed())
		})

		It("should return an error due to duplicates in registry", func() {
			Expect(registry.Add(&corev1.Secret{})).To(Succeed())
			Expect(registry.Add(&corev1.Secret{})).To(MatchError(ContainSubstring("duplicate filename in registry")))
		})

		It("should return an error due to failed serialization", func() {
			registry = NewRegistry(runtime.NewScheme(), codecFactory, serial)

			err := registry.Add(&corev1.Secret{})
			Expect(err).To(HaveOccurred())
			Expect(runtime.IsNotRegisteredError(err)).To(BeTrue())
		})
	})

	Describe("#SerializedObjects", func() {
		It("should return the serialized object map", func() {
			Expect(registry.Add(secret)).To(Succeed())
			Expect(registry.Add(roleBinding)).To(Succeed())

			serializedData := []byte(secretSerialized + "---\n" + roleBindingSerialized)
			compressedData, err := test.BrotliCompression(serializedData)
			Expect(err).NotTo(HaveOccurred())

			Expect(registry.SerializedObjects()).To(Equal(map[string][]byte{
				"data.yaml.br": compressedData,
			}))
		})

		Describe("#AddSerialized", func() {
			It("should add the serialized object", func() {
				registry.AddSerialized("secret__"+secret.Namespace+"__secret_name.yaml", []byte(secretSerialized))

				compressedData, err := test.BrotliCompressionForManifests(secretSerialized)
				Expect(err).NotTo(HaveOccurred())

				Expect(registry.SerializedObjects()).To(Equal(map[string][]byte{
					"data.yaml.br": compressedData,
				}))
			})
		})

		Describe("#AddAllAndSerialize", func() {
			It("should add all objects and return the serialized object map", func() {
				objectMap, err := registry.AddAllAndSerialize(secret, roleBinding)
				Expect(err).NotTo(HaveOccurred())

				compressedData, err := test.BrotliCompressionForManifests(secretSerialized, roleBindingSerialized)
				Expect(err).NotTo(HaveOccurred())

				Expect(objectMap).To(Equal(map[string][]byte{
					"data.yaml.br": compressedData,
				}))
			})
		})

		Describe("#RegisteredObjects", func() {
			It("should return the registered objects", func() {
				Expect(registry.Add(secret)).To(Succeed())
				Expect(registry.Add(roleBinding)).To(Succeed())

				Expect(registry.RegisteredObjects()).To(ConsistOf(roleBinding, secret))
			})
		})

		Describe("#String", func() {
			It("should return the string representation of the registry", func() {
				Expect(registry.Add(secret)).To(Succeed())
				Expect(registry.Add(roleBinding)).To(Succeed())

				result := registry.String()
				Expect(result).To(ContainSubstring(secretSerialized))
				Expect(result).To(ContainSubstring(roleBindingSerialized))
			})
		})
	})
})
