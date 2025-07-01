// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/gardener/operator"
)

var _ = Describe("Extension", func() {
	Describe("#ExtensionAdmissionRuntimeManagedResourceName", func() {
		It("should return the expected managed resource name", func() {
			Expect(ExtensionAdmissionRuntimeManagedResourceName("provider-test")).To(Equal("extension-admission-runtime-provider-test"))
		})
	})

	Describe("#ExtensionAdmissionVirtualManagedResourceName", func() {
		It("should return the expected managed resource name", func() {
			Expect(ExtensionAdmissionVirtualManagedResourceName("provider-test")).To(Equal("extension-admission-virtual-provider-test"))
		})
	})

	Describe("#ExtensionRuntimeManagedResourceName", func() {
		It("should return the expected managed resource name", func() {
			Expect(ExtensionRuntimeManagedResourceName("provider-test")).To(Equal("extension-provider-test-garden"))
		})
	})

	DescribeTable("#ExtensionForManagedResourceName", func(managedResourceName string, expectedExtensionName string, expectedIsExtension bool) {
		extensionName, isExtension := ExtensionForManagedResourceName(managedResourceName)
		Expect(extensionName).To(Equal(expectedExtensionName))
		Expect(isExtension).To(Equal(expectedIsExtension))
	},

		Entry("it should recognize a managed resource of an extension", "extension-foobar-garden", "foobar", true),
		Entry("it should recognize a managed resource of an extension admission for runtime cluster", "extension-admission-runtime-foobar", "foobar", true),
		Entry("it should recognize a managed resource of an extension admission for virtual cluster", "extension-admission-virtual-foobar", "foobar", true),
		Entry("it should not recognize a random managed resource as an extension", "foobar", "", false),
		Entry("it should not recognize a managed resource with a matching prefix only as an extension", "extension-foobar", "", false),
		Entry("it should not recognize a managed resource with a matching suffix only as an extension", "foobar-garden", "", false),
	)

	Describe("#ExtensionRuntimeNamespaceName", func() {
		It("should return the expected namespace name", func() {
			Expect(ExtensionRuntimeNamespaceName("provider-test")).To(Equal("runtime-extension-provider-test"))
		})
	})

	Describe("#IsControllerInstallationInVirtualRequired", func() {
		It("should return true if the extension requires a controller installation in the virtual cluster", func() {
			Expect(IsControllerInstallationInVirtualRequired(&operatorv1alpha1.Extension{
				Status: operatorv1alpha1.ExtensionStatus{
					Conditions: []gardencorev1beta1.Condition{{Type: "RequiredVirtual", Status: "True"}},
				},
			})).To(BeTrue())
		})
	})

	Describe("#IsExtensionInRuntimeRequired", func() {
		It("should return true if the extension requires a deployment in the runtime cluster", func() {
			Expect(IsExtensionInRuntimeRequired(&operatorv1alpha1.Extension{
				Status: operatorv1alpha1.ExtensionStatus{
					Conditions: []gardencorev1beta1.Condition{{Type: "RequiredRuntime", Status: "True"}},
				},
			})).To(BeTrue())
		})
	})

	Describe("#ControllerRegistrationForExtension", func() {
		var extension *operatorv1alpha1.Extension

		BeforeEach(func() {
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: operatorv1alpha1.ExtensionSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind: extensionsv1alpha1.ExtensionResource,
							Type: "test",
						},
						{
							Kind: extensionsv1alpha1.InfrastructureResource,
							Type: "local",
						},
					},
					Deployment: &operatorv1alpha1.Deployment{
						ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
							DeploymentSpec: operatorv1alpha1.DeploymentSpec{
								Helm: &operatorv1alpha1.ExtensionHelm{
									OCIRepository: &gardencorev1.OCIRepository{
										Ref: ptr.To("garden.local/extension:test"),
									},
								},
							},
							Values: &apiextensionsv1.JSON{
								Raw: []byte(`{"foo":"bar"}`),
							},
							Policy: ptr.To(gardencorev1beta1.ControllerDeploymentPolicyAlways),
							SeedSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"deploy-extension": "test"},
							},
							InjectGardenKubeconfig: ptr.To(true),
						},
					},
				},
			}
		})

		It("should return the ControllerRegistration and ControllerDeployment", func() {
			registration, deployment := ControllerRegistrationForExtension(extension)

			Expect(registration.Name).To(Equal(extension.Name))
			Expect(registration.Spec.Resources).To(Equal(extension.Spec.Resources))
			Expect(registration.Spec.Deployment.Policy).To(HaveValue(Equal(gardencorev1beta1.ControllerDeploymentPolicyAlways)))
			Expect(registration.Spec.Deployment.SeedSelector).To(Equal(extension.Spec.Deployment.ExtensionDeployment.SeedSelector))
			Expect(registration.Spec.Deployment.DeploymentRefs).To(ConsistOf(gardencorev1beta1.DeploymentRef{Name: deployment.Name}))

			Expect(deployment.Name).To(Equal(extension.Name))
			Expect(deployment.Helm.OCIRepository).To(Equal(extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository))
			Expect(deployment.Helm.Values).To(Equal(extension.Spec.Deployment.ExtensionDeployment.Values))
			Expect(deployment.InjectGardenKubeconfig).To(Equal(extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig))
		})

		It("should remove garden from autoEnable and clusterCompatibility", func() {
			extension.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"seed", "garden"}
			extension.Spec.Resources[0].ClusterCompatibility = []gardencorev1beta1.ClusterType{"shoot", "seed", "garden"}

			registration, _ := ControllerRegistrationForExtension(extension)

			Expect(registration.Spec.Resources[0].AutoEnable).To(ConsistOf(gardencorev1beta1.ClusterType("seed")))
			Expect(registration.Spec.Resources[0].ClusterCompatibility).To(ConsistOf(gardencorev1beta1.ClusterType("shoot"), gardencorev1beta1.ClusterType("seed")))
		})

		It("should copy the security.gardener.cloud/pod-security-enforce annotation", func() {
			metav1.SetMetaDataAnnotation(&extension.ObjectMeta, "security.gardener.cloud/pod-security-enforce", "true")

			registration, _ := ControllerRegistrationForExtension(extension)

			Expect(registration.Annotations).To(HaveKeyWithValue("security.gardener.cloud/pod-security-enforce", "true"))
		})
	})
})
