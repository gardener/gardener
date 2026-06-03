// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chart_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/chart"
)

var _ = Describe("Values", func() {
	const namespace = "garden"

	var (
		ctx context.Context
		c   client.Client

		secret    *corev1.Secret
		configMap *corev1.ConfigMap
	)

	BeforeEach(func() {
		ctx = context.Background()
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-secret",
				Namespace: namespace,
				Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleResourceReference},
			},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("s3cret"),
			},
		}
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-config",
				Namespace: namespace,
				Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleResourceReference},
			},
			Data: map[string]string{
				"foo": "bar",
				"bar": "foo",
			},
		}
		c = fake.NewClientBuilder().WithObjects(secret, configMap).Build()
	})

	Describe("#ResolveResources", func() {
		It("should resolve Secret and ConfigMap references", func() {
			refs := []gardencorev1.NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
				{Name: "cfg", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "my-config"}},
			}

			result, err := ResolveResources(ctx, c, namespace, refs)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(Resources{
				"creds": ResourceData{Data: map[string]string{"username": "admin", "password": "s3cret"}},
				"cfg":   ResourceData{Data: map[string]string{"foo": "bar", "bar": "foo"}},
			}))
		})

		It("should reject empty apiVersion", func() {
			refs := []gardencorev1.NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "my-secret"}},
			}
			_, err := ResolveResources(ctx, c, namespace, refs)
			Expect(err).To(MatchError(ContainSubstring("unsupported apiVersion")))
		})

		It("should reject unsupported apiVersion", func() {
			refs := []gardencorev1.NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Secret", Name: "my-secret"}},
			}
			_, err := ResolveResources(ctx, c, namespace, refs)
			Expect(err).To(MatchError(ContainSubstring("unsupported apiVersion")))
		})

		It("should reject unsupported kind", func() {
			refs := []gardencorev1.NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Pod", Name: "anything"}},
			}
			_, err := ResolveResources(ctx, c, namespace, refs)
			Expect(err).To(MatchError(ContainSubstring("unsupported kind")))
		})

		It("should report missing Secret", func() {
			refs := []gardencorev1.NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "absent"}},
			}
			_, err := ResolveResources(ctx, c, namespace, refs)
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("should reject Secret without resource-reference label", func() {
			secret.Labels = nil
			c = fake.NewClientBuilder().WithObjects(secret, configMap).Build()

			refs := []gardencorev1.NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
			}
			_, err := ResolveResources(ctx, c, namespace, refs)
			Expect(err).To(MatchError(ContainSubstring("does not have the label")))
		})

		It("should reject ConfigMap without resource-reference label", func() {
			configMap.Labels = nil
			c = fake.NewClientBuilder().WithObjects(secret, configMap).Build()

			refs := []gardencorev1.NamedResourceReference{
				{Name: "cfg", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "my-config"}},
			}
			_, err := ResolveResources(ctx, c, namespace, refs)
			Expect(err).To(MatchError(ContainSubstring("does not have the label")))
		})
	})

	Describe("#SubstituteTemplateInValues", func() {
		It("should substitute resource references in nested strings", func() {
			values := map[string]any{
				"top": "{{ .resources.creds.data.username }}",
				"nested": map[string]any{
					"password": "p:{{ .resources.creds.data.password }}",
					"untouched": map[string]any{
						"int":  42,
						"bool": true,
					},
				},
				"list": []any{
					"{{ .resources.creds.data.username }}",
					"static",
				},
			}
			refs := Resources{
				"creds": ResourceData{Data: map[string]string{"username": "admin", "password": "s3cret"}},
			}

			result, err := SubstituteTemplateInValues(values, refs)
			Expect(err).ToNot(HaveOccurred())
			Expect(result["top"]).To(Equal("admin"))
			Expect(result["nested"].(map[string]any)["password"]).To(Equal("p:s3cret"))
			Expect(result["nested"].(map[string]any)["untouched"]).To(Equal(map[string]any{"int": 42, "bool": true}))
			Expect(result["list"]).To(Equal([]any{"admin", "static"}))
		})

		It("should fail on missing key", func() {
			values := map[string]any{"x": "{{ .resources.unknown.data.foo }}"}
			_, err := SubstituteTemplateInValues(values, Resources{})
			Expect(err).To(HaveOccurred())
		})

		It("should leave non-template strings unchanged", func() {
			values := map[string]any{"x": "plain string"}
			result, err := SubstituteTemplateInValues(values, Resources{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result["x"]).To(Equal("plain string"))
		})
	})

	Describe("ResourceNamesFromValues", func() {
		It("should return an empty set for nil values", func() {
			Expect(ResourceNamesFromValues(nil)).To(Equal(sets.New[string]()))
		})

		It("should return an empty set when no templates are present", func() {
			values := &apiextensionsv1.JSON{Raw: []byte(`{"key":"plain value","nested":{"foo":"bar"}}`)}
			Expect(ResourceNamesFromValues(values)).To(Equal(sets.New[string]()))
		})

		It("should extract resource names from flat values", func() {
			values := &apiextensionsv1.JSON{Raw: []byte(`{"logLevel":"{{ .resources.logLevels.data.extension }}"}`)}
			Expect(ResourceNamesFromValues(values)).To(Equal(sets.New("logLevels")))
		})

		It("should extract resource names from nested values", func() {
			values := &apiextensionsv1.JSON{Raw: []byte(`{"logLevel":"{{ .resources.logLevels.data.extension }}","imageVectorOverwrite":{"images":[{"name":"ccm","ref":"{{ .resources.imageRefs.data.ccm }}"},{"name":"mcm","ref":"{{ .resources.imageRefs.data.mcm }}"}]}}`)}
			Expect(ResourceNamesFromValues(values)).To(Equal(sets.New("logLevels", "imageRefs")))
		})

		It("should deduplicate resource names", func() {
			values := &apiextensionsv1.JSON{Raw: []byte(`{"a":"{{ .resources.foo.data.x }}","b":"{{ .resources.foo.data.y }}"}`)}
			Expect(ResourceNamesFromValues(values)).To(Equal(sets.New("foo")))
		})
	})
})
