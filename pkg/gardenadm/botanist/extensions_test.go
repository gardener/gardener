// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Extensions", func() {
	Describe("#ComputeExtensions", func() {
		var (
			shoot                   *gardencorev1beta1.Shoot
			controllerRegistration1 *gardencorev1beta1.ControllerRegistration
			controllerRegistration2 *gardencorev1beta1.ControllerRegistration
			controllerRegistration3 *gardencorev1beta1.ControllerRegistration
			controllerRegistration4 *gardencorev1beta1.ControllerRegistration
			controllerDeployment1   *gardencorev1.ControllerDeployment
			controllerDeployment2   *gardencorev1.ControllerDeployment
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
				ObjectMeta: metav1.ObjectMeta{Name: "ext1-controlplane"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext1-controlplane"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "ControlPlane", Type: "ext1"},
					},
				},
			}
			controllerRegistration2 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext1-infra-worker"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext1-infra-worker"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "Infrastructure", Type: "ext1"},
						{Kind: "Worker", Type: "ext1"},
					},
				},
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
			controllerRegistration4 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext2"},
			}

			controllerDeployment1 = &gardencorev1.ControllerDeployment{
				ObjectMeta:             metav1.ObjectMeta{Name: "ext1-controlplane"},
				InjectGardenKubeconfig: ptr.To(true),
			}
			controllerDeployment2 = &gardencorev1.ControllerDeployment{
				ObjectMeta:             metav1.ObjectMeta{Name: "ext1-infra-worker"},
				InjectGardenKubeconfig: ptr.To(true),
			}
			controllerDeployment3 = &gardencorev1.ControllerDeployment{
				ObjectMeta:             metav1.ObjectMeta{Name: "ext3"},
				InjectGardenKubeconfig: ptr.To(false),
			}

			controllerRegistrations = []*gardencorev1beta1.ControllerRegistration{controllerRegistration1, controllerRegistration2, controllerRegistration3, controllerRegistration4}
			controllerDeployments = []*gardencorev1.ControllerDeployment{controllerDeployment1, controllerDeployment2, controllerDeployment3}
		})

		It("should return an error because deployment is not set", func() {
			controllerRegistration1.Spec.Deployment = nil

			extensions, err := ComputeExtensions(gardenadm.Resources{
				Shoot:                   shoot,
				ControllerRegistrations: controllerRegistrations,
				ControllerDeployments:   controllerDeployments,
			}, true)
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because more than one deployment ref is set", func() {
			controllerRegistration1.Spec.Deployment.DeploymentRefs = append(controllerRegistration1.Spec.Deployment.DeploymentRefs, gardencorev1beta1.DeploymentRef{})

			extensions, err := ComputeExtensions(gardenadm.Resources{
				Shoot:                   shoot,
				ControllerRegistrations: controllerRegistrations,
				ControllerDeployments:   controllerDeployments,
			}, true)
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because matching ControllerDeployment is not found", func() {
			extensions, err := ComputeExtensions(gardenadm.Resources{
				Shoot:                   shoot,
				ControllerRegistrations: controllerRegistrations,
			}, true)
			Expect(err).To(MatchError(ContainSubstring("was not found")))
			Expect(extensions).To(BeNil())
		})

		When("running the control plane", func() {
			It("should return all extensions referenced by shoot (except Infrastructure and Worker)", func() {
				extensions, err := ComputeExtensions(gardenadm.Resources{
					Shoot:                   shoot,
					ControllerRegistrations: controllerRegistrations,
					ControllerDeployments:   controllerDeployments,
				}, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(extensions).To(Equal([]Extension{
					{
						ControllerRegistration: controllerRegistration1,
						ControllerDeployment:   controllerDeploymentWithoutInjectGardenKubeconfig(controllerDeployment1),
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
						ControllerDeployment:   controllerDeploymentWithoutInjectGardenKubeconfig(controllerDeployment3),
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

		When("not running the control plane", func() {
			It("should return the provider extension only (Infrastructure and Worker)", func() {
				extensions, err := ComputeExtensions(gardenadm.Resources{
					Shoot:                   shoot,
					ControllerRegistrations: controllerRegistrations,
					ControllerDeployments:   controllerDeployments,
				}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(extensions).To(Equal([]Extension{
					{
						ControllerRegistration: controllerRegistration2,
						ControllerDeployment:   controllerDeploymentWithoutInjectGardenKubeconfig(controllerDeployment2),
						ControllerInstallation: &gardencorev1beta1.ControllerInstallation{
							ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration2.Name},
							Spec: gardencorev1beta1.ControllerInstallationSpec{
								RegistrationRef: corev1.ObjectReference{Name: controllerRegistration2.Name},
								DeploymentRef:   &corev1.ObjectReference{Name: controllerDeployment2.Name},
								SeedRef:         corev1.ObjectReference{Name: shoot.Name},
							},
						},
					},
				}))
			})
		})
	})

	Describe("#WaitUntilExtensionControllerInstallationsHealthy", func() {
		var (
			ctx                   = context.Background()
			controlPlaneNamespace = "foo"
			extension1            = "ext1"
			extension2            = "ext2"

			fakeClient client.Client
			b          *AutonomousBotanist

			managedResource1 *resourcesv1alpha1.ManagedResource
			managedResource2 *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithStatusSubresource(&resourcesv1alpha1.ManagedResource{}).Build()
			b = &AutonomousBotanist{
				Botanist: &botanistpkg.Botanist{
					Operation: &operation.Operation{
						SeedClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
						Shoot: &shootpkg.Shoot{
							ControlPlaneNamespace: controlPlaneNamespace,
						},
					},
				},
				Extensions: []Extension{
					{ControllerInstallation: &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: extension1}}},
					{ControllerInstallation: &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: extension2}}},
				},
			}

			managedResource1 = &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: extension1, Namespace: controlPlaneNamespace}}
			managedResource2 = &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: extension2, Namespace: controlPlaneNamespace}}

			DeferCleanup(test.WithVar(&TimeoutManagedResourceHealthCheck, time.Millisecond))
		})

		It("should fail if a ManagedResource does not exist", func() {
			Expect(b.WaitUntilExtensionControllerInstallationsHealthy(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail if a ManagedResource is not healthy", func() {
			Expect(fakeClient.Create(ctx, managedResource1)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource2)).To(Succeed())

			Expect(b.WaitUntilExtensionControllerInstallationsHealthy(ctx)).To(MatchError(ContainSubstring("is not healthy")))
		})

		It("should succeed if all ManagedResource are healthy", func() {
			Expect(fakeClient.Create(ctx, managedResource1)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource2)).To(Succeed())

			Expect(makeManagedResourceHealthy(ctx, fakeClient, managedResource1)).To(Succeed())
			Expect(makeManagedResourceHealthy(ctx, fakeClient, managedResource2)).To(Succeed())

			Expect(b.WaitUntilExtensionControllerInstallationsHealthy(ctx)).To(Succeed())
		})
	})
})

func makeManagedResourceHealthy(ctx context.Context, fakeClient client.Client, mr *resourcesv1alpha1.ManagedResource) error {
	patch := client.MergeFrom(mr.DeepCopy())
	mr.Status.ObservedGeneration = mr.Generation
	mr.Status.Conditions = []gardencorev1beta1.Condition{
		{
			Type:               "ResourcesHealthy",
			Status:             "True",
			LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
			LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
		},
		{
			Type:               "ResourcesApplied",
			Status:             "True",
			LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
			LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
		},
		{
			Type:               "ResourcesProgressing",
			Status:             "False",
			LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
			LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
		},
	}
	return fakeClient.Status().Patch(ctx, mr, patch)
}

func controllerDeploymentWithoutInjectGardenKubeconfig(in *gardencorev1.ControllerDeployment) *gardencorev1.ControllerDeployment {
	out := in.DeepCopy()
	out.InjectGardenKubeconfig = nil
	return out
}
