// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package admissionplugins_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/utils/validation/admissionplugins"
)

var _ = Describe("admissionplugins", func() {
	DescribeTable("#IsAdmissionPluginSupported",
		func(admissionPluginName, version string, supported, success bool) {
			result, err := IsAdmissionPluginSupported(admissionPluginName, version)
			if success {
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(supported))
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry("Unknown admission plugin", "Unknown", "1.25", false, false),
		Entry("Known admission plugin but version not present in supported range", "PodSecurityPolicy", "1.25", false, true),
		Entry("Known admission plugin and version present in supported range", "DenyServiceExternalIPs", "1.25", true, true),
		Entry("Known admission plugin but version range not present", "PodNodeSelector", "1.25", true, true),
	)

	Describe("AdmissionPluginVersionRange", func() {
		DescribeTable("#Contains",
			func(vr AdmissionPluginVersionRange, version string, contains, success bool) {
				result, err := vr.Contains(version)
				if success {
					Expect(err).To(Not(HaveOccurred()))
					Expect(result).To(Equal(contains))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},

			Entry("[,) contains 1.2.3", AdmissionPluginVersionRange{}, "1.2.3", true, true),
			Entry("[,) contains 0.1.2", AdmissionPluginVersionRange{}, "0.1.2", true, true),
			Entry("[,) contains 1.3.5", AdmissionPluginVersionRange{}, "1.3.5", true, true),
			Entry("[,) fails with foo", AdmissionPluginVersionRange{}, "foo", false, false),

			Entry("[, 1.3) contains 1.2.3", AdmissionPluginVersionRange{RemovedInVersion: "1.3"}, "1.2.3", true, true),
			Entry("[, 1.3) contains 0.1.2", AdmissionPluginVersionRange{RemovedInVersion: "1.3"}, "0.1.2", true, true),
			Entry("[, 1.3) doesn't contain 1.3.5", AdmissionPluginVersionRange{RemovedInVersion: "1.3"}, "1.3.5", false, true),
			Entry("[, 1.3) fails with foo", AdmissionPluginVersionRange{RemovedInVersion: "1.3"}, "foo", false, false),

			Entry("[1.0, ) contains 1.2.3", AdmissionPluginVersionRange{AddedInVersion: "1.0"}, "1.2.3", true, true),
			Entry("[1.0, ) doesn't contain 0.1.2", AdmissionPluginVersionRange{AddedInVersion: "1.0"}, "0.1.2", false, true),
			Entry("[1.0, ) contains 1.3.5", AdmissionPluginVersionRange{AddedInVersion: "1.0"}, "1.3.5", true, true),
			Entry("[1.0, ) fails with foo", AdmissionPluginVersionRange{AddedInVersion: "1.0"}, "foo", false, false),

			Entry("[1.0, 1.3) contains 1.2.3", AdmissionPluginVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "1.2.3", true, true),
			Entry("[1.0, 1.3) doesn't contain 0.1.2", AdmissionPluginVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "0.1.2", false, true),
			Entry("[1.0, 1.3) doesn't contain 1.3.5", AdmissionPluginVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "1.3.5", false, true),
			Entry("[1.0, 1.3) fails with foo", AdmissionPluginVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "foo", false, false),
		)
	})

	Describe("#ValidateAdmissionPlugins", func() {
		DescribeTable("validate admission plugins",
			func(plugins []core.AdmissionPlugin, version string, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateAdmissionPlugins(plugins, version, field.NewPath("admissionPlugins"))
				Expect(errList).To(matcher)
			},
			Entry("empty list", nil, "1.18.14", BeEmpty()),
			Entry("supported admission plugin", []core.AdmissionPlugin{{Name: "AlwaysAdmit"}}, "1.18.14", BeEmpty()),
			Entry("unsupported admission plugin", []core.AdmissionPlugin{{Name: "DenyEscalatingExec"}}, "1.22.16", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0].name").String()),
				"Detail": Equal("admission plugin \"DenyEscalatingExec\" is not supported in Kubernetes version 1.22.16"),
			})))),
			Entry("admission plugin without name", []core.AdmissionPlugin{{}}, "1.18.14", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal(field.NewPath("admissionPlugins[0].name").String()),
				"Detail": Equal("must provide a name"),
			})))),
			Entry("unknown admission plugin", []core.AdmissionPlugin{{Name: "Foo"}}, "1.18.14", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal(field.NewPath("admissionPlugins[0].name").String()),
				"BadValue": Equal("Foo"),
				"Detail":   Equal("unknown admission plugin \"Foo\""),
			})))),
			Entry("disabling non-required admission plugin", []core.AdmissionPlugin{{Name: "AlwaysAdmit", Disabled: pointer.Bool(true)}}, "1.18.14", BeEmpty()),
			Entry("disabling required admission plugin", []core.AdmissionPlugin{{Name: "MutatingAdmissionWebhook", Disabled: pointer.Bool(true)}}, "1.18.14", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0]").String()),
				"Detail": Equal("admission plugin \"MutatingAdmissionWebhook\" cannot be disabled"),
			})))),
			Entry("adding forbidden admission plugin", []core.AdmissionPlugin{{Name: "SecurityContextDeny"}}, "1.18.4", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0].name").String()),
				"Detail": Equal("forbidden admission plugin was specified - do not use plugins from the following list: [SecurityContextDeny]"),
			})))),
		)

		Describe("validate PodSecurity admissionPlugin config", func() {
			test := func(kubernetesVersion string, v1alpha1, v1beta1, v1 bool) {
				if v1alpha1 {
					It("should allow v1alpha1 configuration", func() {
						Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
							getPodSecurityPluginForConfigVersion("v1alpha1"),
						},
							kubernetesVersion,
							field.NewPath("admissionPlugins"),
						)).To(BeEmpty())
					})
				}

				if v1beta1 {
					It("should allow v1beta1 configuration", func() {
						Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
							getPodSecurityPluginForConfigVersion("v1beta1"),
						},
							kubernetesVersion,
							field.NewPath("admissionPlugins"),
						)).To(BeEmpty())
					})
				} else {
					It("should not allow v1beta1 configuration", func() {
						Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
							getPodSecurityPluginForConfigVersion("v1beta1"),
						},
							kubernetesVersion,
							field.NewPath("admissionPlugins"),
						)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal(field.NewPath("admissionPlugins[0].config").String()),
							"Detail": ContainSubstring("PodSecurityConfiguration apiVersion for Kubernetes version %q should be %q but got %q", kubernetesVersion, "pod-security.admission.config.k8s.io/v1alpha1", "pod-security.admission.config.k8s.io/v1beta1"),
						}))))
					})
				}

				if v1 {
					It("should allow v1 configuration", func() {
						Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
							getPodSecurityPluginForConfigVersion("v1"),
						},
							kubernetesVersion,
							field.NewPath("admissionPlugins"),
						)).To(BeEmpty())
					})
				} else if v1alpha1 && v1beta1 {
					It("should not allow v1 configuration", func() {
						Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
							getPodSecurityPluginForConfigVersion("v1"),
						},
							kubernetesVersion,
							field.NewPath("admissionPlugins"),
						)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal(field.NewPath("admissionPlugins[0].config").String()),
							"Detail": ContainSubstring("PodSecurityConfiguration apiVersion for Kubernetes version %q should be %q but got %q", kubernetesVersion, "pod-security.admission.config.k8s.io/v1beta1 or pod-security.admission.config.k8s.io/v1alpha1", "pod-security.admission.config.k8s.io/v1"),
						}))))
					})
				} else {
					It("should not allow v1 configuration", func() {
						Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
							getPodSecurityPluginForConfigVersion("v1"),
						},
							kubernetesVersion,
							field.NewPath("admissionPlugins"),
						)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal(field.NewPath("admissionPlugins[0].config").String()),
							"Detail": ContainSubstring("PodSecurityConfiguration apiVersion for Kubernetes version %q should be %q but got %q", kubernetesVersion, "pod-security.admission.config.k8s.io/v1alpha1", "pod-security.admission.config.k8s.io/v1"),
						}))))
					})
				}
			}

			Context("v1.22 cluster", func() {
				test("v1.22.13", true, false, false)
			})
			Context("v1.23 cluster", func() {
				test("v1.23.10", true, true, false)
			})
			Context("v1.24 cluster", func() {
				test("v1.24.8", true, true, false)
			})
			Context("v1.25 cluster", func() {
				test("v1.25.4", true, true, true)
			})
			Context("v1.26 cluster", func() {
				test("v1.26.2", true, true, true)
			})

			Context("invalid PodSecurityConfiguration", func() {
				It("should return error if decoding fails", func() {
					Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
						{
							Name: "PodSecurity",
							Config: &runtime.RawExtension{Raw: []byte(`apiVersion: pod-security.admission.config.k8s.io/foo
kind: PodSecurityConfiguration-bar
defaults:
   enforce-error: "privileged"
 enforce-version: "latest"
 exemptions:
usernames: "admin"
`),
							},
						},
					},
						"v1.24.8",
						field.NewPath("admissionPlugins"),
					)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(field.NewPath("admissionPlugins[0].config").String()),
						"Detail": ContainSubstring("cannot decode the given config: yaml: line 4: did not find expected key"),
					}))))
				})

				It("should return error non-registered error if wrong apiVersion is passed", func() {
					Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
						{
							Name: "PodSecurity",
							Config: &runtime.RawExtension{Raw: []byte(`apiVersion: pod-security.admission.config.k8s.io/foo
kind: PodSecurityConfiguration-bar
defaults:
  enforce-error: "privileged"
  enforce-version: "latest"
exemptions:
  usernames: "admin"
`),
							},
						},
					},
						"v1.24.8",
						field.NewPath("admissionPlugins"),
					)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(field.NewPath("admissionPlugins[0].config").String()),
						"Detail": ContainSubstring("expected pod-security.admission.config.k8s.io/v1alpha1.PodSecurityConfiguration or pod-security.admission.config.k8s.io/v1beta1.PodSecurityConfiguration or pod-security.admission.config.k8s.io/v1.PodSecurityConfiguration"),
					}))))
				})
			})
		})
	})
})

func getPodSecurityPluginForConfigVersion(version string) core.AdmissionPlugin {
	apiVersion := "pod-security.admission.config.k8s.io/v1alpha1"

	if version == "v1beta1" {
		apiVersion = "pod-security.admission.config.k8s.io/v1beta1"
	} else if version == "v1" {
		apiVersion = "pod-security.admission.config.k8s.io/v1"
	}

	return core.AdmissionPlugin{
		Name: "PodSecurity",
		Config: &runtime.RawExtension{Raw: []byte(`apiVersion: ` + apiVersion + `
kind: PodSecurityConfiguration
defaults:
  enforce: "privileged"
  enforce-version: "latest"
  audit-version: "latest"
  warn: "baseline"
  warn-version: "v1.22"
exemptions:
  usernames: ["admin"]
  runtimeClasses: ["random"]
  namespaces: ["random"]
`),
		},
	}
}
