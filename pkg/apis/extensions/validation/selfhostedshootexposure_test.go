// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("SelfHostedShootExposure validation tests", func() {
	var exposure *extensionsv1alpha1.SelfHostedShootExposure

	BeforeEach(func() {
		exposure = &extensionsv1alpha1.SelfHostedShootExposure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-exposure",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.SelfHostedShootExposureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           "provider",
					ProviderConfig: &runtime.RawExtension{},
				},
				CredentialsRef: &corev1.ObjectReference{
					Name: "test",
				},
				Endpoints: []extensionsv1alpha1.ControlPlaneEndpoint{
					{
						NodeName: "test-node",
						Port:     443,
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "1.2.3.4",
							},
						},
					},
				},
			},
		}
	})

	Describe("#ValidSelfHostedShootExposure", func() {
		It("should forbid empty SelfHostedShootExposure resources", func() {
			errorList := ValidateSelfHostedShootExposure(&extensionsv1alpha1.SelfHostedShootExposure{})

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
				"Field": Equal("spec.endpoints"),
			}))))
		})

		It("should allow valid infra resources", func() {
			Expect(ValidateSelfHostedShootExposure(exposure)).To(BeEmpty())
		})

		It("should forbid endpoints with invalid port", func() {
			e := prepareSelfHostedShootExposureForUpdate(exposure)
			e.Spec.Endpoints[0].Port = 70000

			errorList := ValidateSelfHostedShootExposure(e)

			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.endpoints[0].port"),
			}))))
		})

		It("should forbid endpoints with invalid IP address", func() {
			e := prepareSelfHostedShootExposureForUpdate(exposure)
			e.Spec.Endpoints[0].Addresses = []corev1.NodeAddress{{
				Type:    corev1.NodeInternalIP,
				Address: "not-an-ip",
			}}

			errorList := ValidateSelfHostedShootExposure(e)

			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.endpoints[0].addresses[0].address"),
			}))))
		})

		It("should forbid endpoints with invalid hostname for NodeHostName type", func() {
			e := prepareSelfHostedShootExposureForUpdate(exposure)
			e.Spec.Endpoints[0].Addresses = []corev1.NodeAddress{{
				Type:    corev1.NodeHostName,
				Address: "invalid_host!",
			}}

			errorList := ValidateSelfHostedShootExposure(e)

			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.endpoints[0].addresses[0].address"),
			}))))
		})

		It("should forbid unknown node address types", func() {
			e := prepareSelfHostedShootExposureForUpdate(exposure)
			e.Spec.Endpoints[0].Addresses = []corev1.NodeAddress{{
				Type:    "UnknownType",
				Address: "1.2.3.4",
			}}

			errorList := ValidateSelfHostedShootExposure(e)

			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.endpoints[0].addresses[0].type"),
			}))))
		})
	})

	Describe("#ValidSelfHostedShootExposureUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			exposure.DeletionTimestamp = &now

			newSelfHostedShootExposure := prepareSelfHostedShootExposureForUpdate(exposure)
			newSelfHostedShootExposure.DeletionTimestamp = &now
			newSelfHostedShootExposure.Spec.CredentialsRef.Name = "changed-secretref-name"

			errorList := ValidateSelfHostedShootExposureUpdate(newSelfHostedShootExposure, exposure)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update SelfHostedShootExposure spec if deletion timestamp is set. Requested changes: CredentialsRef.Name: changed-secretref-name != test"),
			}))))
		})

		It("should prevent updating the type and region", func() {
			newSelfHostedShootExposure := prepareSelfHostedShootExposureForUpdate(exposure)
			newSelfHostedShootExposure.Spec.Type = "changed-type"

			errorList := ValidateSelfHostedShootExposureUpdate(newSelfHostedShootExposure, exposure)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			}))))
		})

		It("should allow updating the name of the referenced secret, the provider config, or the endpoints", func() {
			newSelfHostedShootExposure := prepareSelfHostedShootExposureForUpdate(exposure)
			newSelfHostedShootExposure.Spec.CredentialsRef.Name = "changed-secretref-name"
			newSelfHostedShootExposure.Spec.ProviderConfig = nil
			newSelfHostedShootExposure.Spec.Endpoints = []extensionsv1alpha1.ControlPlaneEndpoint{
				{
					NodeName: "update-node",
					Port:     443,
					Addresses: []corev1.NodeAddress{
						{
							Type: corev1.NodeInternalIP,
						},
					},
				},
			}

			errorList := ValidateSelfHostedShootExposureUpdate(newSelfHostedShootExposure, exposure)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareSelfHostedShootExposureForUpdate(obj *extensionsv1alpha1.SelfHostedShootExposure) *extensionsv1alpha1.SelfHostedShootExposure {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
