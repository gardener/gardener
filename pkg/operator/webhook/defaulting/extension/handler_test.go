// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/webhook/defaulting/extension"
)

var _ = Describe("Handler", func() {
	var (
		ctx       context.Context
		handler   *Handler
		extension *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		ctx = context.Background()
		handler = &Handler{}
		extension = &operatorv1alpha1.Extension{
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: "Worker", Type: "test"},
				},
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						InjectGardenKubeconfig: ptr.To(true),
					},
				},
			},
		}
	})

	Describe("#Default", func() {
		Context("injectGardenKubeconfig defaulting", func() {
			It("should do nothing if the extension does not handle Worker resources", func() {
				extension.Spec.Resources = nil
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = nil

				Expect(handler.Default(ctx, extension)).To(Succeed())
				Expect(extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig).To(BeNil())
			})

			It("should do nothing if the deployment section is not set", func() {
				extension.Spec.Deployment = nil

				Expect(handler.Default(ctx, extension)).To(Succeed())
				Expect(extension.Spec.Deployment).To(BeNil())
			})

			It("should do nothing if the extension deployment section is not set", func() {
				extension.Spec.Deployment.ExtensionDeployment = nil

				Expect(handler.Default(ctx, extension)).To(Succeed())
				Expect(extension.Spec.Deployment.ExtensionDeployment).To(BeNil())
			})

			It("should do nothing if injectGardenKubeconfig is already set", func() {
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = ptr.To(false)

				Expect(handler.Default(ctx, extension)).To(Succeed())
				Expect(extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig).To(PointTo(BeFalse()))
			})

			It("should do default the injectGardenKubeconfig to true", func() {
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = nil

				Expect(handler.Default(ctx, extension)).To(Succeed())
				Expect(extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig).To(PointTo(BeTrue()))
			})
		})

		Context("primary defaulting", func() {
			It("should default the primary field to true", func() {
				Expect(handler.Default(ctx, extension)).To(Succeed())
				Expect(extension.Spec.Resources).To(ConsistOf(
					gardencorev1beta1.ControllerResource{
						Kind:    "Worker",
						Type:    "test",
						Primary: ptr.To(true),
					},
				))
			})

			It("should not overwrite the primary field", func() {
				extension.Spec.Resources[0].Primary = ptr.To(false)

				Expect(handler.Default(ctx, extension)).To(Succeed())
				Expect(extension.Spec.Resources).To(ConsistOf(
					gardencorev1beta1.ControllerResource{
						Kind:    "Worker",
						Type:    "test",
						Primary: ptr.To(false),
					},
				))
			})
		})

		Context("autoEnable defaulting", func() {
			Context("kind == Extension", func() {
				BeforeEach(func() {
					extension.Spec.Resources[0].Kind = "Extension"
				})

				It("should default the autoEnable field to none", func() {
					Expect(handler.Default(ctx, extension)).To(Succeed())
					Expect(extension.Spec.Resources).To(ConsistOf(
						gardencorev1beta1.ControllerResource{
							Kind:       "Extension",
							Type:       "test",
							Primary:    ptr.To(true),
							AutoEnable: nil,
						},
					))
				})

				It("should default the autoEnable field to shoot", func() {
					extension.Spec.Resources[0].GloballyEnabled = ptr.To(true)

					Expect(handler.Default(ctx, extension)).To(Succeed())
					Expect(extension.Spec.Resources).To(ConsistOf(
						gardencorev1beta1.ControllerResource{
							Kind:            "Extension",
							Type:            "test",
							Primary:         ptr.To(true),
							GloballyEnabled: ptr.To(true),
							AutoEnable:      []gardencorev1beta1.AutoEnableMode{"shoot"},
						},
					))
				})

				It("should not overwrite the autoEnable field if already set", func() {
					extension.Spec.Resources[0].GloballyEnabled = ptr.To(true)
					extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.AutoEnableMode{"seed"}

					Expect(handler.Default(ctx, extension)).To(Succeed())
					Expect(extension.Spec.Resources).To(ConsistOf(
						gardencorev1beta1.ControllerResource{
							Kind:            "Extension",
							Type:            "test",
							Primary:         ptr.To(true),
							GloballyEnabled: ptr.To(true),
							AutoEnable:      []gardencorev1beta1.AutoEnableMode{"seed"},
						},
					))
				})
			})

			Context("kind != Extension", func() {
				It("should not overwrite the autoEnable field", func() {
					Expect(handler.Default(ctx, extension)).To(Succeed())
					Expect(extension.Spec.Resources).To(ConsistOf(
						gardencorev1beta1.ControllerResource{
							Kind:       "Worker",
							Type:       "test",
							Primary:    ptr.To(true),
							AutoEnable: nil,
						},
					))
				})
			})
		})
	})
})
