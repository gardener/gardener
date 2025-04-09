// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
)

var _ = Describe("Extensions", func() {
	Describe("#ComputeExtensions", func() {
		var (
			seedName               = "test"
			controllerRegistration *gardencorev1beta1.ControllerRegistration
			controllerDeployment   *gardencorev1.ControllerDeployment
		)

		BeforeEach(func() {
			controllerRegistration = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext1"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext1"},
						},
					},
				},
			}
			controllerDeployment = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "ext1"},
			}
		})

		It("should return an error because deployment is not set", func() {
			controllerRegistration.Spec.Deployment = nil

			extensions, err := ComputeExtensions(seedName, []*gardencorev1beta1.ControllerRegistration{controllerRegistration}, []*gardencorev1.ControllerDeployment{controllerDeployment})
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because more than one deployment ref is set", func() {
			controllerRegistration.Spec.Deployment.DeploymentRefs = append(controllerRegistration.Spec.Deployment.DeploymentRefs, gardencorev1beta1.DeploymentRef{})

			extensions, err := ComputeExtensions(seedName, []*gardencorev1beta1.ControllerRegistration{controllerRegistration}, []*gardencorev1.ControllerDeployment{controllerDeployment})
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because matching ControllerDeployment is not found", func() {
			extensions, err := ComputeExtensions(seedName, []*gardencorev1beta1.ControllerRegistration{controllerRegistration}, nil)
			Expect(err).To(MatchError(ContainSubstring("was not found")))
			Expect(extensions).To(BeNil())
		})

		It("should return the computed extensions", func() {
			extensions, err := ComputeExtensions(seedName, []*gardencorev1beta1.ControllerRegistration{controllerRegistration}, []*gardencorev1.ControllerDeployment{controllerDeployment})
			Expect(err).NotTo(HaveOccurred())
			Expect(extensions).To(Equal([]Extension{
				{
					ControllerRegistration: controllerRegistration,
					ControllerDeployment:   controllerDeployment,
					ControllerInstallation: &gardencorev1beta1.ControllerInstallation{
						ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration.Name},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{Name: controllerRegistration.Name},
							DeploymentRef:   &corev1.ObjectReference{Name: controllerDeployment.Name},
							SeedRef:         corev1.ObjectReference{Name: seedName},
						},
					},
				},
			}))
		})
	})
})
