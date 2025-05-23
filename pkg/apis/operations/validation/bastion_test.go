// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/operations"
	. "github.com/gardener/gardener/pkg/apis/operations/validation"
)

var _ = Describe("validation", func() {
	var bastion *operations.Bastion

	BeforeEach(func() {
		bastion = &operations.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example-bastion",
				Namespace: "garden",
			},
			Spec: operations.BastionSpec{
				ShootRef: corev1.LocalObjectReference{
					Name: "example-shoot",
				},
				SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== you@example.com",
				Ingress: []operations.BastionIngressPolicy{{
					IPBlock: networkingv1.IPBlock{
						CIDR: "1.2.3.4/8",
					},
				}},
			},
		}
	})

	Describe("#ValidateBastion", func() {
		It("should not return any errors", func() {
			errorList := ValidateBastion(bastion)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid Bastion resources with empty metadata", func() {
			bastion.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateBastion(bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			}))))
		})

		It("should forbid Bastion specification with empty SSH key", func() {
			bastion.Spec.SSHPublicKey = ""

			errorList := ValidateBastion(bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.sshPublicKey"),
			}))))
		})

		It("should forbid Bastion specification with invalid SSH key", func() {
			bastion.Spec.SSHPublicKey = "i-am-not-a-valid-ssh-key"

			errorList := ValidateBastion(bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.sshPublicKey"),
			}))))
		})

		It("should forbid Bastion specification with empty Shoot ref", func() {
			bastion.Spec.ShootRef.Name = ""

			errorList := ValidateBastion(bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.shootRef.name"),
			}))))
		})

		It("should forbid Bastion specification with empty ingress", func() {
			bastion.Spec.Ingress = []operations.BastionIngressPolicy{}

			errorList := ValidateBastion(bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.ingress"),
			}))))
		})

		It("should forbid Bastion specification with invalid ingress", func() {
			bastion.Spec.Ingress[0].IPBlock.CIDR = "not-a-cidr"

			errorList := ValidateBastion(bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.ingress"),
			}))))
		})

		It("should forbid changing Shoot ref", func() {
			newBastion := prepareBastionForUpdate(bastion)
			newBastion.Spec.ShootRef.Name = "another-shoot"

			errorList := ValidateBastionUpdate(newBastion, bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.shootRef.name"),
			}))))
		})

		It("should forbid changing SSH key", func() {
			newBastion := prepareBastionForUpdate(bastion)
			newBastion.Spec.SSHPublicKey += "-adding-a-suffix"

			errorList := ValidateBastionUpdate(newBastion, bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.sshPublicKey"),
			}))))
		})
	})
})

func prepareBastionForUpdate(obj *operations.Bastion) *operations.Bastion {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"

	return newObj
}
