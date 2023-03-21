// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("OperatingSystemConfig validation tests", func() {
	var osc *extensionsv1alpha1.OperatingSystemConfig

	BeforeEach(func() {
		reloadConfigFilePath := "some-path"

		osc = &extensionsv1alpha1.OperatingSystemConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-osc",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           "provider",
					ProviderConfig: &runtime.RawExtension{},
				},
				Purpose:              extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
				ReloadConfigFilePath: &reloadConfigFilePath,
				Units: []extensionsv1alpha1.Unit{
					{
						Name: "foo",
						DropIns: []extensionsv1alpha1.DropIn{
							{
								Name:    "drop1",
								Content: "data1",
							},
						},
					},
				},
				Files: []extensionsv1alpha1.File{
					{
						Path: "foo/bar",
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data:     "some-data",
							},
						},
					},
				},
			},
		}
	})

	Describe("#ValidOperatingSystemConfig", func() {
		It("should forbid empty OperatingSystemConfig resources", func() {
			errorList := ValidateOperatingSystemConfig(&extensionsv1alpha1.OperatingSystemConfig{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should forbid OperatingSystemConfig resources with invalid purpose", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Purpose = extensionsv1alpha1.OperatingSystemConfigPurpose("foo")

			errorList := ValidateOperatingSystemConfig(oscCopy)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should forbid OperatingSystemConfig resources with invalid units", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Units[0].Name = ""
			oscCopy.Spec.Units[0].DropIns[0].Name = ""
			oscCopy.Spec.Units[0].DropIns[0].Content = ""

			errorList := ValidateOperatingSystemConfig(oscCopy)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.units[0].name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.units[0].dropIns[0].name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.units[0].dropIns[0].content"),
			}))))
		})

		It("should forbid OperatingSystemConfig resources with invalid files", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Files = []extensionsv1alpha1.File{
				{},
				{
					Path: "path1",
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{},
						Inline:    &extensionsv1alpha1.FileContentInline{},
					},
				},
				{
					Path: "path2",
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{},
					},
				},
				{
					Path: "path3",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "foo",
						},
					},
				},
				{
					Path:    "path3",
					Content: osc.Spec.Files[0].Content,
				},
			}

			errorList := ValidateOperatingSystemConfig(oscCopy)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.files[0].path"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.files[0].content"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.files[1].content"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.files[2].content.secretRef.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.files[2].content.secretRef.dataKey"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.files[3].content.inline.encoding"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.files[3].content.inline.data"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("spec.files[4].path"),
			}))))
		})

		It("should allow valid osc resources", func() {
			errorList := ValidateOperatingSystemConfig(osc)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidOperatingSystemConfigUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			osc.DeletionTimestamp = &now

			newOperatingSystemConfig := prepareOperatingSystemConfigForUpdate(osc)
			newOperatingSystemConfig.DeletionTimestamp = &now
			newOperatingSystemConfig.Spec.Type = "changed-type"

			errorList := ValidateOperatingSystemConfigUpdate(newOperatingSystemConfig, osc)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("DefaultSpec.Type: changed-type != provider"),
			}))))
		})

		It("should prevent updating the type and purpose", func() {
			newOperatingSystemConfig := prepareOperatingSystemConfigForUpdate(osc)
			newOperatingSystemConfig.Spec.Type = "changed-type"
			newOperatingSystemConfig.Spec.Purpose = extensionsv1alpha1.OperatingSystemConfigPurposeReconcile

			errorList := ValidateOperatingSystemConfigUpdate(newOperatingSystemConfig, osc)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should allow updating the units, files, and the provider config", func() {
			newOperatingSystemConfig := prepareOperatingSystemConfigForUpdate(osc)
			newOperatingSystemConfig.Spec.ProviderConfig = nil
			newOperatingSystemConfig.Spec.Units = nil
			newOperatingSystemConfig.Spec.Files = nil

			errorList := ValidateOperatingSystemConfigUpdate(newOperatingSystemConfig, osc)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareOperatingSystemConfigForUpdate(obj *extensionsv1alpha1.OperatingSystemConfig) *extensionsv1alpha1.OperatingSystemConfig {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
