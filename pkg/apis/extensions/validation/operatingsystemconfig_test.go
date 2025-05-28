// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("OperatingSystemConfig validation tests", func() {
	var osc *extensionsv1alpha1.OperatingSystemConfig

	BeforeEach(func() {
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
				Purpose: extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
				CRIConfig: &extensionsv1alpha1.CRIConfig{
					Name: extensionsv1alpha1.CRINameContainerD,
					Containerd: &extensionsv1alpha1.ContainerdConfig{
						Registries: []extensionsv1alpha1.RegistryConfig{
							{
								Upstream: "docker.io",
								Server:   ptr.To("https://docker.io"),
								Hosts: []extensionsv1alpha1.RegistryHost{
									{
										URL:          "https://registry-1.docker.io",
										Capabilities: []extensionsv1alpha1.RegistryCapability{"pull", "resolve"},
									},
								},
							},
						},
						SandboxImage: "pause",
						Plugins: []extensionsv1alpha1.PluginConfig{
							{
								Path: []string{"io.containerd.grpc.v1.cri", "registry", "configs", "gcr.io", "auth"},
								Values: &apiextensionsv1.JSON{
									Raw: []byte(`{"username": "foo"}`),
								},
							},
						},
					},
				},
				Units: []extensionsv1alpha1.Unit{
					{
						Name: "foo",
						DropIns: []extensionsv1alpha1.DropIn{
							{
								Name:    "drop1",
								Content: "data1",
							},
						},
						FilePaths: []string{"/foo-bar-file"},
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
					{
						Path: "/foo-bar-file",
						Content: extensionsv1alpha1.FileContent{
							ImageRef: &extensionsv1alpha1.FileContentImageRef{
								Image:           "foo-image:bar-tag",
								FilePathInImage: "/foo-bar-file",
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
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should forbid OperatingSystemConfig resources with invalid purpose", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Purpose = "foo"

			errorList := ValidateOperatingSystemConfig(oscCopy)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should forbid OperatingSystemConfig resources with invalid units", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Units = []extensionsv1alpha1.Unit{{
				DropIns: []extensionsv1alpha1.DropIn{
					{},
				},
				FilePaths: []string{"non-existing-foobar"},
			}}
			oscCopy.Status.ExtensionUnits = []extensionsv1alpha1.Unit{{
				DropIns: []extensionsv1alpha1.DropIn{
					{},
				},
			}}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.units[0].name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.units[0].dropIns[0].name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.units[0].dropIns[0].content"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.units[0].filePaths[0]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionUnits[0].name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionUnits[0].dropIns[0].name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionUnits[0].dropIns[0].content"),
				})),
			))
		})

		It("should forbid OperatingSystemConfig resources with invalid files", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Units = nil
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
			oscCopy.Status.ExtensionFiles = []extensionsv1alpha1.File{
				{},
				{
					Path: "path4",
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{},
						Inline:    &extensionsv1alpha1.FileContentInline{},
					},
				},
				{
					Path: "path5",
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{},
					},
				},
				{
					Path: "path6",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "foo",
						},
					},
				},
				{
					Path:    "path6",
					Content: osc.Spec.Files[0].Content,
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
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
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionFiles[0].path"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionFiles[0].content"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.extensionFiles[1].content"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionFiles[2].content.secretRef.name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionFiles[2].content.secretRef.dataKey"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("status.extensionFiles[3].content.inline.encoding"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("status.extensionFiles[3].content.inline.data"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("status.extensionFiles[4].path"),
				})),
			))
		})

		It("should forbid an empty OperatingSystemConfigs plugin path", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Units = nil
			oscCopy.Spec.Files = []extensionsv1alpha1.File{{
				Path: "path1",
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     "some-data",
					},
				},
			}}
			oscCopy.Status.ExtensionFiles = oscCopy.Spec.Files

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("status.extensionFiles[0].path"),
			}))))
		})

		It("should forbid setting an unknown cgroup driver", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.CgroupDriver = ptr.To(extensionsv1alpha1.CgroupDriverName("unknown"))

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.criConfig.cgroupDriver"),
			}))))
		})

		It("should allow setting a known cgroup driver", func() {
			for _, driver := range []extensionsv1alpha1.CgroupDriverName{"cgroupfs", "systemd"} {
				oscCopy := osc.DeepCopy()
				oscCopy.Spec.CRIConfig.CgroupDriver = ptr.To(driver)

				Expect(ValidateOperatingSystemConfig(oscCopy)).To(BeEmpty(), driver+" should be configurable")
			}
		})

		It("should allow setting CRI name for provisioning OSC", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Purpose = extensionsv1alpha1.OperatingSystemConfigPurposeProvision
			oscCopy.Spec.CRIConfig.Containerd = nil

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(BeEmpty())
		})

		It("should forbid setting containerd config for provisioning OSC", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.Purpose = extensionsv1alpha1.OperatingSystemConfigPurposeProvision

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.criConfig.containerd"),
				"Detail": Equal(`containerd config is not allowed for OperatingSystemConfig with purpose 'provision'`),
			}))))
		})

		It("should forbid containerd config with duplicated registry host", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = append(oscCopy.Spec.CRIConfig.Containerd.Registries, oscCopy.Spec.CRIConfig.Containerd.Registries[0])

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeDuplicate),
				"Field":    Equal("spec.criConfig.containerd.registries[1].upstream"),
				"BadValue": Equal("docker.io"),
			}))))
		})

		It("should forbid containerd config with an invalid upstream name", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "a/b.io",
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("spec.criConfig.containerd.registries[0].upstream"),
				"BadValue": Equal("a/b.io"),
			}))))
		})

		It("should forbid containerd config with an upstream that contains a scheme", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "http://foo.bar",
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("spec.criConfig.containerd.registries[0].upstream"),
				"BadValue": Equal("http://foo.bar"),
			}))))
		})

		It("should forbid containerd config with an upstream that contains a path", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "foo.bar/baz",
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("spec.criConfig.containerd.registries[0].upstream"),
				"BadValue": Equal("foo.bar/baz"),
			}))))
		})

		It("should allow containerd config with an upstream that contains a port", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "foo.bar:5001",
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(BeEmpty())
		})

		It("should allow containerd config with _default upstream name", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "_default",
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(BeEmpty())
		})

		It("should forbid containerd with an invalid server name", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "foo.bar",
					Server:   ptr.To("ftp://foo.bar"),
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeNotSupported),
				"Field":    Equal("spec.criConfig.containerd.registries[0].server"),
				"BadValue": Equal("ftp"),
			}))))
		})

		It("should forbid containerd with an invalid hosts url", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "foo.bar",
					Hosts: []extensionsv1alpha1.RegistryHost{
						{
							URL: "1a",
						},
					},
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.criConfig.containerd.registries[0].hosts[0].url"),
			}))))
		})

		It("should forbid containerd with invalid hosts capabilities", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{
				{
					Upstream: "foo.bar",
					Hosts: []extensionsv1alpha1.RegistryHost{
						{
							URL:          "http://foo.bar/us",
							Capabilities: []extensionsv1alpha1.RegistryCapability{"foo"},
						},
					},
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeNotSupported),
				"Field":    Equal("spec.criConfig.containerd.registries[0].hosts[0].capabilities[0]"),
				"BadValue": Equal(extensionsv1alpha1.RegistryCapability("foo")),
			}))))
		})

		It("should forbid containerd with empty sandbox image", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.SandboxImage = ""

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.criConfig.containerd.sandboxImage"),
			}))))
		})

		It("should forbid OperatingSystemConfig with an invalid plugin path operation", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
				{
					Op:   ptr.To[extensionsv1alpha1.PluginPathOperation]("invalid-op"),
					Path: []string{"foo"},
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.criConfig.containerd.plugins[0].op"),
			}))))
		})

		It("should allow OperatingSystemConfig with an valid plugin path operation", func() {
			for _, op := range []extensionsv1alpha1.PluginPathOperation{"add", "remove"} {
				oscCopy := osc.DeepCopy()
				oscCopy.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
					{
						Op:   ptr.To(op),
						Path: []string{"foo"},
					},
				}

				Expect(ValidateOperatingSystemConfig(oscCopy)).To(BeEmpty(), op+" should be configurable")
			}
		})

		It("should forbid OperatingSystemConfig with an empty plugin path", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
				{
					Path: []string{},
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.criConfig.containerd.plugins[0].path"),
			}))))
		})

		It("should forbid OperatingSystemConfig with invalid plugin values", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
				{
					Path: []string{"foo"},
					Values: &apiextensionsv1.JSON{
						Raw: []byte(`[1]`),
					},
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.criConfig.containerd.plugins[0].values"),
			}))))
		})

		It("should forbid OperatingSystemConfig with plugin values when remove operation is used", func() {
			oscCopy := osc.DeepCopy()
			oscCopy.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
				{
					Path: []string{"foo"},
					Op:   ptr.To[extensionsv1alpha1.PluginPathOperation]("remove"),
					Values: &apiextensionsv1.JSON{
						Raw: []byte(`[1]`),
					},
				},
			}

			Expect(ValidateOperatingSystemConfig(oscCopy)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.criConfig.containerd.plugins[0].values"),
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
				"Detail": Equal("cannot update operatingsystemconfig spec if deletion timestamp is set. Requested changes: DefaultSpec.Type: changed-type != provider"),
			}))))
		})

		It("should prevent updating the type and purpose", func() {
			newOperatingSystemConfig := prepareOperatingSystemConfigForUpdate(osc)
			newOperatingSystemConfig.Spec.Type = "changed-type"
			newOperatingSystemConfig.Spec.Purpose = extensionsv1alpha1.OperatingSystemConfigPurposeProvision

			errorList := ValidateOperatingSystemConfigUpdate(newOperatingSystemConfig, osc)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.type"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.purpose"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.criConfig.containerd"),
					"Detail": Equal(`containerd config is not allowed for OperatingSystemConfig with purpose 'provision'`),
				})),
			))
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
