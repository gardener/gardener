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

	v1 "github.com/gardener/gardener/pkg/apis/core/v1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/utils"
)

var (
	//go:embed extensions.yaml
	extensionsYAML string
	extensions     map[string]Extension
)

var _ = Describe("ExtensionDefaulter", func() {
	var (
		extName     string
		operatorExt *operatorv1alpha1.Extension
	)
	BeforeEach(func() {
		extName = "provider-local"
		operatorExt = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extName,
			},
		}
	})

	Describe("#NotFound", func() {
		It("should return the extension as-is", func() {
			// rename extension to an unknown name
			operatorExt = &operatorv1alpha1.Extension{
				TypeMeta: metav1.TypeMeta{},
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
			_, ok := ExtensionSpecFor(operatorExt.Name)
			Expect(ok).To(BeFalse())

			ext, err := MergeExtensionSpecs(*operatorExt)
			Expect(err).NotTo(HaveOccurred())
			Expect(ext).To(Equal(operatorExt))
		})
	})
	Describe("#WellKnownExtensions", func() {
		It("should default zero fields", func() {
			// rename extension to an unknown name
			ext, err := MergeExtensionSpecs(*operatorExt)
			Expect(err).NotTo(HaveOccurred())
			Expect(ext.Spec.Deployment.ExtensionDeployment.DeploymentSpec.Helm.OCIRepository.Ref).NotTo(BeNil())
		})
		It("should respect populated fields", func() {
			// validate test conditions
			extFromDefaults, ok := ExtensionSpecFor(operatorExt.Name)
			Expect(ok).To(BeTrue())
			Expect(*extFromDefaults.Deployment.ExtensionDeployment.DeploymentSpec.Helm.OCIRepository.Ref).NotTo(Equal("foo"))
			Expect(operatorExt.Spec.Deployment).To(BeNil())

			// test
			operatorExt.Spec.Deployment = &operatorv1alpha1.Deployment{
				ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &v1.OCIRepository{
								Ref: ptr.To("foo"),
							},
						},
					},
				},
			}
			// rename extension to an unknown name
			ext, err := MergeExtensionSpecs(*operatorExt)
			Expect(err).NotTo(HaveOccurred())
			Expect(*ext.Spec.Deployment.ExtensionDeployment.DeploymentSpec.Helm.OCIRepository.Ref).To(Equal("foo"))
		})
	})
})
