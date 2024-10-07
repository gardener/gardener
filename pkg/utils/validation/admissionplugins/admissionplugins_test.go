// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admissionplugins_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

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
		Entry("Unknown admission plugin", "Unknown", "1.30", false, false),
		Entry("Known admission plugin but version not present in supported range", "SecurityContextDeny", "1.30", false, true),
		Entry("Known admission plugin and version present in supported range", "DenyServiceExternalIPs", "1.30", true, true),
		Entry("Known admission plugin but version range not present", "PodNodeSelector", "1.30", true, true),
	)

	Describe("#ValidateAdmissionPlugins", func() {
		DescribeTable("validate admission plugins",
			func(plugins []core.AdmissionPlugin, version string, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateAdmissionPlugins(plugins, version, field.NewPath("admissionPlugins"))
				Expect(errList).To(matcher)
			},
			Entry("empty list", nil, "1.27.1", BeEmpty()),
			Entry("supported admission plugin", []core.AdmissionPlugin{{Name: "AlwaysAdmit"}}, "1.27.1", BeEmpty()),
			Entry("unsupported admission plugin", []core.AdmissionPlugin{{Name: "PersistentVolumeLabel"}}, "1.31.1", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0].name").String()),
				"Detail": Equal("admission plugin \"PersistentVolumeLabel\" is not supported in Kubernetes version 1.31.1"),
			})))),
			Entry("unsupported admission plugin but is disabled", []core.AdmissionPlugin{{Name: "PersistentVolumeLabel", Disabled: ptr.To(true)}}, "1.31.1", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0].name").String()),
				"Detail": Equal("admission plugin \"PersistentVolumeLabel\" is not supported in Kubernetes version 1.31.1"),
			})))),
			Entry("admission plugin without name", []core.AdmissionPlugin{{}}, "1.29.10", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal(field.NewPath("admissionPlugins[0].name").String()),
				"Detail": Equal("must provide a name"),
			})))),
			Entry("unknown admission plugin", []core.AdmissionPlugin{{Name: "Foo"}}, "1.29.8", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal(field.NewPath("admissionPlugins[0].name").String()),
				"BadValue": Equal("Foo"),
				"Detail":   Equal("unknown admission plugin \"Foo\""),
			})))),
			Entry("disabling non-required admission plugin", []core.AdmissionPlugin{{Name: "AlwaysAdmit", Disabled: ptr.To(true)}}, "1.29.8", BeEmpty()),
			Entry("disabling required admission plugin", []core.AdmissionPlugin{{Name: "MutatingAdmissionWebhook", Disabled: ptr.To(true)}}, "1.29.8", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0]").String()),
				"Detail": Equal("admission plugin \"MutatingAdmissionWebhook\" cannot be disabled"),
			})))),
			Entry("adding forbidden admission plugin", []core.AdmissionPlugin{{Name: "SecurityContextDeny"}}, "1.27.4", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0].name").String()),
				"Detail": Equal("forbidden admission plugin was specified - do not use plugins from the following list: [SecurityContextDeny]"),
			})))),
			Entry("adding kubeconfig secret to admission plugin not supporting external kubeconfig", []core.AdmissionPlugin{{Name: "TaintNodesByCondition", KubeconfigSecretName: ptr.To("test-secret")}}, "1.27.5", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("admissionPlugins[0].kubeconfigSecretName").String()),
				"Detail": Equal("admission plugin \"TaintNodesByCondition\" does not allow specifying external kubeconfig"),
			})))),
			Entry("adding kubeconfig secret to admission plugin supporting external kubeconfig", []core.AdmissionPlugin{{Name: "ValidatingAdmissionWebhook", KubeconfigSecretName: ptr.To("test-secret")}}, "1.27.5", BeEmpty()),
		)

		Describe("validate PodSecurity admissionPlugin config", func() {
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
						"1.31.1",
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
						"1.31.1",
						field.NewPath("admissionPlugins"),
					)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(field.NewPath("admissionPlugins[0].config").String()),
						"Detail": ContainSubstring("expected pod-security.admission.config.k8s.io/v1.PodSecurityConfiguration"),
					}))))
				})

				It("should not return an error with valid configuration", func() {
					Expect(ValidateAdmissionPlugins([]core.AdmissionPlugin{
						{
							Name: "PodSecurity",
							Config: &runtime.RawExtension{Raw: []byte(`apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce-error: "privileged"
  enforce-version: "latest"
exemptions:
  usernames: ["admin"]
`),
							},
						},
					},
						"1.31.1",
						field.NewPath("admissionPlugins"),
					)).To(BeEmpty())
				})
			})
		})
	})
})
