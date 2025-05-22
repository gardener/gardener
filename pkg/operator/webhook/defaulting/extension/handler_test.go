// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/webhook/defaulting/extension"
)

var _ = Describe("Handler", func() {
	var (
		ctx     context.Context
		encoder runtime.Encoder

		handler   *Handler
		extension *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		ctx = context.Background()
		encoder = &json.Serializer{}

		handler = &Handler{Decoder: admission.NewDecoder(operatorclient.RuntimeScheme)}
		extension = &operatorv1alpha1.Extension{
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: "Worker", Type: "test", Primary: ptr.To(true)},
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
			It("should not default if the extension does not handle Worker resources", func() {
				extension.Spec.Resources = nil
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = nil

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should not default if the deployment section is not set", func() {
				extension.Spec.Deployment = nil

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should not default if the extension deployment section is not set", func() {
				extension.Spec.Deployment.ExtensionDeployment = nil

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should not default if injectGardenKubeconfig is already set", func() {
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = ptr.To(false)

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should default the injectGardenKubeconfig to true", func() {
				extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = nil

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "add",
							Path:      "/spec/deployment/extension/injectGardenKubeconfig",
							Value:     true,
						},
					},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed:   true,
						PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
					},
				}))
			})
		})

		Context("primary defaulting", func() {
			It("should default the primary field to true", func() {
				extension.Spec.Resources[0].Primary = nil

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "add",
							Path:      "/spec/resources/0/primary",
							Value:     true,
						},
					},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed:   true,
						PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
					},
				}))
			})

			It("should not overwrite the primary field", func() {
				extension.Spec.Resources[0].Primary = ptr.To(false)

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})
		})

		Context("autoEnable defaulting", func() {
			When("kind == Extension", func() {
				BeforeEach(func() {
					extension.Spec.Resources[0].Kind = "Extension"
				})

				It("should not default the autoEnable field", func() {
					Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
						Patches: []jsonpatch.JsonPatchOperation{},
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed: true,
						},
					}))
				})

				It("should not default autoEnable field if configured for seed", func() {
					extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"seed"}

					Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
						Patches: []jsonpatch.JsonPatchOperation{},
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed: true,
						},
					}))
				})

				It("should default the autoEnable field to shoot", func() {
					extension.Spec.Resources[0].GloballyEnabled = ptr.To(true)

					Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
						Patches: []jsonpatch.JsonPatchOperation{
							{
								Operation: "add",
								Path:      "/spec/resources/0/autoEnable",
								Value:     []interface{}{"shoot"},
							},
							{
								Operation: "add",
								Path:      "/spec/resources/0/clusterCompatibility",
								Value:     []interface{}{"shoot"},
							},
						},
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed:   true,
							PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
						},
					}))
				})

				It("should add shoot to autoEnable field if globallyEnabled is set to true", func() {
					extension.Spec.Resources[0].GloballyEnabled = ptr.To(true)
					extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"seed"}

					Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
						Patches: []jsonpatch.JsonPatchOperation{
							{
								Operation: "add",
								Path:      "/spec/resources/0/autoEnable/1",
								Value:     "shoot",
							},
							{
								Operation: "add",
								Path:      "/spec/resources/0/clusterCompatibility",
								Value:     []interface{}{"shoot"},
							},
						},
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed:   true,
							PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
						},
					}))
				})

				It("should remove shoot from autoEnable field if globallyEnabled is updated to false", func() {
					extension.Spec.Resources[0].GloballyEnabled = ptr.To(false)
					extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"shoot", "seed"}

					extensionOld := extension.DeepCopy()
					extensionOld.Spec.Resources[0].GloballyEnabled = ptr.To(true)

					Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Update, OldObject: runtime.RawExtension{Raw: mustEncodeObject(encoder, extensionOld)}, Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
						Patches: []jsonpatch.JsonPatchOperation{
							{
								Operation: "remove",
								Path:      "/spec/resources/0/autoEnable/1",
							},
							{
								Operation: "replace",
								Path:      "/spec/resources/0/autoEnable/0",
								Value:     "seed",
							},
						},
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed:   true,
							PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
						},
					}))
				})
			})

			When("kind != Extension", func() {
				It("should not default the autoEnable field", func() {
					Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
						Patches: []jsonpatch.JsonPatchOperation{},
						AdmissionResponse: admissionv1.AdmissionResponse{
							Allowed: true,
						},
					}))
				})
			})
		})

		Context("globallyEnabled defaulting", func() {
			BeforeEach(func() {
				extension.Spec.Resources[0].Kind = "Extension"
				extension.Spec.Resources[0].ClusterCompatibility = []gardencorev1beta1.ClusterType{"shoot"}
			})

			It("should not default the globallyEnabled field", func() {
				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should not default the globallyEnabled field if autoEnable does not contain shoot", func() {
				extension.Spec.Resources[0].GloballyEnabled = ptr.To(false)
				extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"seed"}

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should not default the globallyEnabled field if it is unused", func() {
				extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"shoot"}

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should default the globallyEnabled field if it is used", func() {
				extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"shoot"}
				extension.Spec.Resources[0].GloballyEnabled = ptr.To(false)

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "replace",
							Path:      "/spec/resources/0/globallyEnabled",
							Value:     true,
						},
					},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed:   true,
						PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
					},
				}))
			})
		})

		Context("clusterCompatibility defaulting", func() {
			BeforeEach(func() {
				extension.Spec.Resources[0].Kind = "Extension"
			})

			It("should not default the clusterCompatibility field", func() {
				extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"seed"}

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}))
			})

			It("should default the clusterCompatibility field to shoot", func() {
				extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"shoot"}

				Expect(handler.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustEncodeObject(encoder, extension)}}})).To(Equal(admission.Response{
					Patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "add",
							Path:      "/spec/resources/0/clusterCompatibility",
							Value:     []interface{}{"shoot"},
						},
					},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed:   true,
						PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
					},
				}))
			})

		})
	})
})

func mustEncodeObject(encoder runtime.Encoder, obj runtime.Object) []byte {
	GinkgoHelper()
	data, err := runtime.Encode(encoder, obj)
	Expect(err).ToNot(HaveOccurred())
	return data
}
