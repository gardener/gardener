//  SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	_ "embed"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/utils"
)

var _ = Describe("ExtensionDefaulter", func() {
	var (
		extensionName string
		extension     *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		extensionName = "provider-local"
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionName,
			},
		}
	})

	Describe("#ExtensionSpecFor", func() {
		It("should return the extension as-is", func() {
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: operatorv1alpha1.ExtensionSpec{
					Deployment: &operatorv1alpha1.Deployment{
						ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
							Values: &apiextensionsv1.JSON{
								Raw: []byte(`{"foo":"bar"}`),
							},
						},
					},
				},
			}
			_, ok := ExtensionSpecFor(extension.Name)
			Expect(ok).To(BeFalse())

			appliedExtension := extension.DeepCopy()
			Expect(ApplyExtensionSpec(appliedExtension)).NotTo(HaveOccurred())
			Expect(appliedExtension).To(Equal(extension))
		})
	})

	Describe("#ApplyExtensionSpec", func() {
		It("should default zero fields", func() {
			Expect(ApplyExtensionSpec(extension)).NotTo(HaveOccurred())
			Expect(extension.Spec.Deployment.ExtensionDeployment.DeploymentSpec.Helm.OCIRepository.Ref).NotTo(BeNil())
		})

		It("should respect populated fields", func() {
			// validate test conditions
			extFromDefaults, ok := ExtensionSpecFor(extension.Name)
			Expect(ok).To(BeTrue())
			Expect(*extFromDefaults.Deployment.ExtensionDeployment.DeploymentSpec.Helm.OCIRepository.Ref).NotTo(Equal("foo"))
			Expect(extension.Spec.Deployment).To(BeNil())

			extension.Spec.Deployment = &operatorv1alpha1.Deployment{
				ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{
								Ref: ptr.To("foo"),
							},
						},
					},
				},
			}
			Expect(ApplyExtensionSpec(extension)).NotTo(HaveOccurred())
			Expect(*extension.Spec.Deployment.ExtensionDeployment.DeploymentSpec.Helm.OCIRepository.Ref).To(Equal("foo"))
		})
	})
})
