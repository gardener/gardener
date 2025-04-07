// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/settings"
	settingsvalidation "github.com/gardener/gardener/pkg/apis/settings/validation"
)

type clusterOpenIDConnectPresetProvider struct {
	new *settings.ClusterOpenIDConnectPreset
}

func (o *clusterOpenIDConnectPresetProvider) providerFunc() field.ErrorList {
	return settingsvalidation.ValidateClusterOpenIDConnectPreset(o.new)
}
func (o *clusterOpenIDConnectPresetProvider) preset() settings.Preset { return o.new }

type clusterOpenIDConnectPresetUpdateProvider struct {
	new *settings.ClusterOpenIDConnectPreset
	old *settings.ClusterOpenIDConnectPreset
}

func (o *clusterOpenIDConnectPresetUpdateProvider) providerFunc() field.ErrorList {
	return settingsvalidation.ValidateClusterOpenIDConnectPresetUpdate(o.new, o.old)
}
func (o *clusterOpenIDConnectPresetUpdateProvider) preset() settings.Preset { return o.new }

var _ = Describe("ClusterOpenIDConnectPreset", func() {

	var preset *settings.ClusterOpenIDConnectPreset

	BeforeEach(func() {
		preset = &settings.ClusterOpenIDConnectPreset{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: settings.ClusterOpenIDConnectPresetSpec{
				OpenIDConnectPresetSpec: settings.OpenIDConnectPresetSpec{
					Weight: 1,
					Server: settings.KubeAPIServerOpenIDConnect{
						IssuerURL: "https://foo.bar",
						ClientID:  "client-baz",
					},
				},
			},
		}

	})

	Describe("#ValidateClusterOpenIDConnectPreset", func() {
		provider := &clusterOpenIDConnectPresetProvider{}

		BeforeEach(func() {
			provider.new = preset.DeepCopy()
		})

		It("should forbid empty object", func() {

			provider.new.Name = ""
			provider.new.Namespace = ""
			provider.new.Spec = settings.ClusterOpenIDConnectPresetSpec{}

			errorList := provider.providerFunc()

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.weight"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.server.issuerURL"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.server.clientID"),
			})),
			))
		})

		validationAssertions(provider)

		Context("projectSelector", func() {
			It("invalid selector", func() {
				provider.new.Spec.ProjectSelector = &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: metav1.LabelSelectorOpExists,
							Values:   []string{"bar"},
						}},
				}

				errorList := provider.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.projectSelector.matchExpressions[0].values"),
					"Detail": Equal("may not be specified when `operator` is 'Exists' or 'DoesNotExist'"),
				})),
				))
			})
		})

	})

	Describe("#ValidateClusterOpenIDConnectPresetUpdate", func() {
		provider := &clusterOpenIDConnectPresetUpdateProvider{}

		BeforeEach(func() {
			provider.old = preset.DeepCopy()
			provider.old.ResourceVersion = "2"

			provider.new = preset.DeepCopy()
			provider.new.ResourceVersion = "2"
		})

		It("should forbid update with mutation of objectmeta fields", func() {

			provider.new.Name = "changed-name"
			provider.new.ResourceVersion = ""

			errorList := provider.providerFunc()

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("metadata.name"),
				"Detail": Equal("field is immutable"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("metadata.resourceVersion"),
				"Detail": Equal("must be specified for an update"),
			})),
			))
		})

		validationAssertions(provider)
	})
})
