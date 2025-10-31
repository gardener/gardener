// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			shoot *gardencorev1beta1.Shoot

			controllerRegistrationControlPlane *gardencorev1beta1.ControllerRegistration
			controllerRegistrationInfraWorker  *gardencorev1beta1.ControllerRegistration
			controllerRegistrationNetwork      *gardencorev1beta1.ControllerRegistration
			controllerRegistrationOSC          *gardencorev1beta1.ControllerRegistration
			controllerRegistrationDNS          *gardencorev1beta1.ControllerRegistration
			controllerRegistrationUnused       *gardencorev1beta1.ControllerRegistration

			controllerDeploymentControlPlane *gardencorev1.ControllerDeployment
			controllerDeploymentInfraWorker  *gardencorev1.ControllerDeployment
			controllerDeploymentNetwork      *gardencorev1.ControllerDeployment
			controllerDeploymentOSC          *gardencorev1.ControllerDeployment
			controllerDeploymentDNS          *gardencorev1.ControllerDeployment

			controllerRegistrations []*gardencorev1beta1.ControllerRegistration
			controllerDeployments   []*gardencorev1.ControllerDeployment

			resources gardenadm.Resources
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: "ext1",
						Workers: []gardencorev1beta1.Worker{{
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
							Machine: gardencorev1beta1.Machine{
								Image: &gardencorev1beta1.ShootMachineImage{
									Name: "ext-osc",
								},
							},
						}},
					},
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To("ext-network"),
					},
					DNS: &gardencorev1beta1.DNS{
						Domain: ptr.To("foo.gardener.cloud"),
						Providers: []gardencorev1beta1.DNSProvider{
							{
								Type:       ptr.To("clouddns"),
								Primary:    ptr.To(true),
								SecretName: ptr.To("dns-credentials"),
							},
							{
								Type:       ptr.To("unused"),
								SecretName: ptr.To("dns-credentials-unused"),
							},
						},
					},
				},
			}
			controllerRegistrationControlPlane = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-controlplane"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext-controlplane"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "ControlPlane", Type: "ext1"},
					},
				},
			}
			controllerRegistrationInfraWorker = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-infra-worker"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext-infra-worker"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "Infrastructure", Type: "ext1"},
						{Kind: "Worker", Type: "ext1"},
					},
				},
			}
			controllerRegistrationNetwork = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-network"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext-network"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "Network", Type: "ext-network"},
					},
				},
			}
			controllerRegistrationOSC = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-osc"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "ext-osc"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "OperatingSystemConfig", Type: "ext-osc"},
					},
				},
			}
			controllerRegistrationDNS = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "dns-clouddns"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{
							{Name: "dns-clouddns"},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "DNSRecord", Type: "clouddns"},
					},
				},
			}
			controllerRegistrationUnused = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-unused"},
			}

			controllerDeploymentControlPlane = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-controlplane"},
			}
			controllerDeploymentInfraWorker = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-infra-worker"},
			}
			controllerDeploymentNetwork = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-network"},
			}
			controllerDeploymentOSC = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "ext-osc"},
			}
			controllerDeploymentDNS = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "dns-clouddns"},
			}

			controllerRegistrations = []*gardencorev1beta1.ControllerRegistration{
				controllerRegistrationControlPlane,
				controllerRegistrationInfraWorker,
				controllerRegistrationNetwork,
				controllerRegistrationOSC,
				controllerRegistrationDNS,
				controllerRegistrationUnused,
			}
			controllerDeployments = []*gardencorev1.ControllerDeployment{
				controllerDeploymentControlPlane,
				controllerDeploymentInfraWorker,
				controllerDeploymentNetwork,
				controllerDeploymentOSC,
				controllerDeploymentDNS,
			}

			resources = gardenadm.Resources{
				Shoot:                   shoot,
				ControllerRegistrations: controllerRegistrations,
				ControllerDeployments:   controllerDeployments,
			}
		})

		It("should return an error because deployment is not set", func() {
			controllerRegistrationControlPlane.Spec.Deployment = nil

			extensions, err := ComputeExtensions(resources, true, true)
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because more than one deployment ref is set", func() {
			controllerRegistrationControlPlane.Spec.Deployment.DeploymentRefs = append(controllerRegistrationControlPlane.Spec.Deployment.DeploymentRefs, gardencorev1beta1.DeploymentRef{})

			extensions, err := ComputeExtensions(resources, true, true)
			Expect(err).To(MatchError(ContainSubstring("has invalid deployment refs in its spec")))
			Expect(extensions).To(BeNil())
		})

		It("should return an error because matching ControllerDeployment is not found", func() {
			resources.ControllerDeployments = nil

			extensions, err := ComputeExtensions(resources, true, true)
			Expect(err).To(MatchError(ContainSubstring("was not found")))
			Expect(extensions).To(BeNil())
		})

		It("should correctly construct the ControllerInstallation", func() {
			Expect(ComputeExtensions(resources, true, false)).To(ContainElement(
				HaveField("ControllerInstallation", And(
					HaveField("ObjectMeta.Name", controllerRegistrationControlPlane.Name),
					HaveField("Spec.RegistrationRef.Name", controllerRegistrationControlPlane.Name),
					HaveField("Spec.DeploymentRef.Name", controllerDeploymentControlPlane.Name),
					HaveField("Spec.SeedRef.Name", shoot.Name),
				)),
			))
		})

		When("running the control plane (gardenadm init)", func() {
			When("infrastructure is not managed by Gardener", func() {
				It("should return all extensions referenced by shoot (except Infrastructure, Worker, and DNSRecord)", func() {
					Expect(ComputeExtensions(resources, true, false)).To(ConsistOf(
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationControlPlane.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentControlPlane.Name),
						),
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationOSC.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentOSC.Name),
						),
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationNetwork.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentNetwork.Name),
						),
					))
				})
			})

			When("infrastructure is managed by Gardener", func() {
				It("should return all extensions referenced by shoot", func() {
					Expect(ComputeExtensions(resources, true, true)).To(ConsistOf(
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationControlPlane.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentControlPlane.Name),
						),
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationOSC.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentOSC.Name),
						),
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationNetwork.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentNetwork.Name),
						),
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationInfraWorker.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentInfraWorker.Name),
						),
						And(
							HaveField("ControllerRegistration.Name", controllerRegistrationDNS.Name),
							HaveField("ControllerDeployment.Name", controllerDeploymentDNS.Name),
						),
					))
				})
			})
		})

		When("not running the control plane (gardenadm bootstrap)", func() {
			It("should return the Infrastructure, Worker, OSC, and DNSRecord extensions", func() {
				Expect(ComputeExtensions(resources, false, true)).To(ConsistOf(
					And(
						HaveField("ControllerRegistration.Name", controllerRegistrationInfraWorker.Name),
						HaveField("ControllerDeployment.Name", controllerDeploymentInfraWorker.Name),
					),
					And(
						HaveField("ControllerRegistration.Name", controllerRegistrationOSC.Name),
						HaveField("ControllerDeployment.Name", controllerDeploymentOSC.Name),
					),
					And(
						HaveField("ControllerRegistration.Name", controllerRegistrationDNS.Name),
						HaveField("ControllerDeployment.Name", controllerDeploymentDNS.Name),
					),
				))
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
			b          *GardenadmBotanist

			managedResource1 *resourcesv1alpha1.ManagedResource
			managedResource2 *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithStatusSubresource(&resourcesv1alpha1.ManagedResource{}).Build()
			b = &GardenadmBotanist{
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
