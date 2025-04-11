// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
)

var _ = Describe("Extensions", func() {
	Describe("#ComputeExtensions", func() {
		var (
			shoot                   *gardencorev1beta1.Shoot
			controllerRegistration1 *gardencorev1beta1.ControllerRegistration
			controllerRegistration2 *gardencorev1beta1.ControllerRegistration
			controllerRegistration3 *gardencorev1beta1.ControllerRegistration
			controllerDeployment1   *gardencorev1.ControllerDeployment
			controllerDeployment3   *gardencorev1.ControllerDeployment

			controllerRegistrations []*gardencorev1beta1.ControllerRegistration
			controllerDeployments   []*gardencorev1.ControllerDeployment
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type:    "ext1",
						Workers: []gardencorev1beta1.Worker{{ControlPlane: &gardencorev1beta1.WorkerControlPlane{}}},
					},
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To("ext3"),
					},
				},
			}
			controllerRegistration1 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext1"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext1"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "ControlPlane", Type: "ext1"},
					},
				},
			}
			controllerRegistration2 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext2"},
			}
			controllerRegistration3 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext3"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext3"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "Network", Type: "ext3"},
					},
				},
			}
			controllerDeployment1 = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "ext1"},
			}
			controllerDeployment3 = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "ext3"},
			}

			controllerRegistrations = []*gardencorev1beta1.ControllerRegistration{controllerRegistration1, controllerRegistration2, controllerRegistration3}
			controllerDeployments = []*gardencorev1.ControllerDeployment{controllerDeployment1, controllerDeployment3}
		})

		It("should return an error because deployment is not set", func() {
			controllerRegistration1.Spec.Deployment = nil

			extensions, err := ComputeExtensions(shoot, controllerRegistrations, controllerDeployments)
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because more than one deployment ref is set", func() {
			controllerRegistration1.Spec.Deployment.DeploymentRefs = append(controllerRegistration1.Spec.Deployment.DeploymentRefs, gardencorev1beta1.DeploymentRef{})

			extensions, err := ComputeExtensions(shoot, controllerRegistrations, controllerDeployments)
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because matching ControllerDeployment is not found", func() {
			extensions, err := ComputeExtensions(shoot, controllerRegistrations, nil)
			Expect(err).To(MatchError(ContainSubstring("was not found")))
			Expect(extensions).To(BeNil())
		})

		It("should return the computed extensions", func() {
			extensions, err := ComputeExtensions(shoot, controllerRegistrations, controllerDeployments)
			Expect(err).NotTo(HaveOccurred())
			Expect(extensions).To(Equal([]Extension{
				{
					ControllerRegistration: controllerRegistration1,
					ControllerDeployment:   controllerDeployment1,
					ControllerInstallation: &gardencorev1beta1.ControllerInstallation{
						ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration1.Name},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{Name: controllerRegistration1.Name},
							DeploymentRef:   &corev1.ObjectReference{Name: controllerDeployment1.Name},
							SeedRef:         corev1.ObjectReference{Name: shoot.Name},
						},
					},
				},
				{
					ControllerRegistration: controllerRegistration3,
					ControllerDeployment:   controllerDeployment3,
					ControllerInstallation: &gardencorev1beta1.ControllerInstallation{
						ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration3.Name},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{Name: controllerRegistration3.Name},
							DeploymentRef:   &corev1.ObjectReference{Name: controllerDeployment3.Name},
							SeedRef:         corev1.ObjectReference{Name: shoot.Name},
						},
					},
				},
			}))
		})
	})
})
