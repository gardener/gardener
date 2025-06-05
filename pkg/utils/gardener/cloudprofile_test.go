// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("CloudProfile", func() {
	Describe("v1beta1", func() {
		Describe("#GetCloudProfile", func() {
			var (
				ctx        context.Context
				fakeClient client.Client

				namespaceName              string
				cloudProfileName           string
				namespacedCloudProfileName string

				cloudProfile           *gardencorev1beta1.CloudProfile
				namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile

				shoot *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

				ctx = context.Background()

				namespaceName = "foo"
				cloudProfileName = "profile-1"
				namespacedCloudProfileName = "n-profile-1"

				cloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: cloudProfileName,
					},
				}

				namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      namespacedCloudProfileName,
						Namespace: namespaceName,
					},
				}

				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
					},
					Spec: gardencorev1beta1.ShootSpec{},
				}
			})

			It("returns an error if no CloudProfile can be found", func() {
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				_, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(err).To(MatchError(ContainSubstring("cloudprofiles.core.gardener.cloud \"profile-1\" not found")))
			})

			It("returns CloudProfile if present", func() {
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res).To(Equal(cloudProfile))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the CloudProfile from the cloudProfile reference if present despite cloudProfileName", func() {
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfileName = ptr.To("profile-1")
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res).To(Equal(cloudProfile))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the CloudProfile from the cloudProfile reference with empty kind field", func() {
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Name: cloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res).To(Equal(cloudProfile))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns NamespacedCloudProfile", func() {
				Expect(fakeClient.Create(ctx, namespacedCloudProfile)).To(Succeed())

				shoot.Spec.CloudProfileName = &cloudProfileName
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res.Spec).To(Equal(namespacedCloudProfile.Status.CloudProfileSpec))
				Expect(res.Namespace).To(Equal(namespaceName))
				Expect(res.Name).To(Equal(namespacedCloudProfileName))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("core", func() {
		var (
			coreInformerFactory          gardencoreinformers.SharedInformerFactory
			cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
			namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister

			namespaceName              string
			cloudProfileName           string
			namespacedCloudProfileName string

			cloudProfile           *gardencorev1beta1.CloudProfile
			namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile

			shoot *core.Shoot
		)

		BeforeEach(func() {
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			cloudProfileLister = coreInformerFactory.Core().V1beta1().CloudProfiles().Lister()
			namespacedCloudProfileLister = coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Lister()

			namespaceName = "foo"
			cloudProfileName = "profile-1"
			namespacedCloudProfileName = "n-profile-1"

			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
			}

			namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
				Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
					Parent: gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: cloudProfileName,
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedCloudProfileName,
					Namespace: namespaceName,
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
			}
		})
		Describe("#GetCloudProfile", func() {
			It("returns an error if CloudProfile is not found", func() {
				shoot.Spec.CloudProfileName = &cloudProfileName
				res, err := gardenerutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
				Expect(res).To(BeNil())
				Expect(err).To(HaveOccurred())
			})

			It("returns CloudProfile if present, derived from cloudProfileName", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfileName = &cloudProfileName
				res, err := gardenerutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
				Expect(res).To(Equal(&cloudProfile.Spec))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns CloudProfile if present, derived from cloudProfile reference", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Name: cloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
				Expect(res).To(Equal(&cloudProfile.Spec))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns NamespacedCloudProfile if present", func() {
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
				Expect(res).To(Equal(&namespacedCloudProfile.Status.CloudProfileSpec))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not derive a NamespacedCloudProfile from cloudProfileName", func() {
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

				shoot.Spec.CloudProfileName = &namespacedCloudProfileName
				res, err := gardenerutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
				Expect(res).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("#ValidateCloudProfileChanges", func() {
			BeforeEach(func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, false))
			})

			It("should pass if the CloudProfile did not change from cloudProfileName to cloudProfile, without kind", func() {
				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfileName = &cloudProfileName
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Name: cloudProfileName,
				}

				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass if the CloudProfile did not change from cloudProfile to cloudProfile", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				newShoot := shoot.DeepCopy()

				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass if the CloudProfile did not change from cloudProfile to cloudProfileName", func() {
				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				newShoot.Spec.CloudProfileName = &cloudProfileName

				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass if the NamespacedCloudProfile did not change", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				newShoot := shoot.DeepCopy()

				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass if the CloudProfile referenced by cloudProfileName is updated to a direct descendant NamespacedCloudProfile", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfileName = &cloudProfileName
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass if the CloudProfile referenced by cloudProfile is updated to a direct descendant NamespacedCloudProfile", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass if the NamespacedCloudProfile referenced by cloudProfile is updated to a its parent CloudProfile", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass if the NamespacedCloudProfile referenced by cloudProfile is updated to another related NamespacedCloudProfile", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

				anotherNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
				anotherNamespacedCloudProfile.Name = namespacedCloudProfileName + "-2"

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(anotherNamespacedCloudProfile)).To(Succeed())

				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName + "-2",
				}
				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail if the CloudProfile referenced by cloudProfileName is updated to an unrelated NamespacedCloudProfile", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

				unrelatedNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
				unrelatedNamespacedCloudProfile.Spec.Parent = gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "someOtherCloudProfile",
				}

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(unrelatedNamespacedCloudProfile)).To(Succeed())

				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfileName = &cloudProfileName
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: unrelatedNamespacedCloudProfile.Name,
				}
				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).To(HaveOccurred())
			})

			It("should fail if the CloudProfile is updated to another CloudProfile", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

				unrelatedCloudProfile := cloudProfile.DeepCopy()
				unrelatedCloudProfile.Name = "someOtherCloudProfile"

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(unrelatedCloudProfile)).To(Succeed())

				newShoot := shoot.DeepCopy()
				shoot.Spec.CloudProfileName = &cloudProfileName
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: unrelatedCloudProfile.Name,
				}
				err := gardenerutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("#BuildCoreCloudProfileReference", func() {
			It("should return nil for nil shoot", func() {
				Expect(gardenerutils.BuildCoreCloudProfileReference(nil)).To(BeNil())
			})

			It("should build and return cloud profile reference from an existing cloudProfileName", func() {
				Expect(gardenerutils.BuildCoreCloudProfileReference(&core.Shoot{Spec: core.ShootSpec{
					CloudProfileName: ptr.To("profile-name"),
				}})).To(Equal(&gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "profile-name",
				}))
			})

			It("should return an existing cloud profile reference", func() {
				Expect(gardenerutils.BuildCoreCloudProfileReference(&core.Shoot{Spec: core.ShootSpec{
					CloudProfileName: ptr.To("ignore-me"),
					CloudProfile: &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: "profile-1",
					},
				}})).To(Equal(&gardencorev1beta1.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "profile-1",
				}))
			})

			It("should return an existing cloud profile reference and default the kind to CloudProfile", func() {
				Expect(gardenerutils.BuildCoreCloudProfileReference(&core.Shoot{Spec: core.ShootSpec{
					CloudProfileName: ptr.To("ignore-me"),
					CloudProfile: &core.CloudProfileReference{
						Name: "profile-1",
					},
				}})).To(Equal(&gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "profile-1",
				}))
			})
		})

		Describe("#SyncCloudProfileFields", func() {
			BeforeEach(func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, false))
			})

			It("should default the cloudProfile to the cloudProfileName value", func() {
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfileName: ptr.To("profile")}}
				gardenerutils.SyncCloudProfileFields(nil, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			It("should default the cloudProfileName to the cloudProfile value and to kind CloudProfile", func() {
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfile: &core.CloudProfileReference{Name: "profile"}}}
				gardenerutils.SyncCloudProfileFields(nil, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			It("should override the cloudProfileName from the cloudProfile", func() {
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfileName: ptr.To("profile-name"), CloudProfile: &core.CloudProfileReference{Name: "profile"}}}
				gardenerutils.SyncCloudProfileFields(nil, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			It("should override cloudProfile from cloudProfileName as with disabledFeatureToggle reference to NamespacedCloudProfile is ignored", func() {
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfileName: ptr.To("profile"), CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}}}
				gardenerutils.SyncCloudProfileFields(nil, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			It("should remove the cloudProfileName if a NamespacedCloudProfile is given and the feature is enabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfileName: ptr.To("profile"), CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}}}
				gardenerutils.SyncCloudProfileFields(nil, shoot)
				Expect(shoot.Spec.CloudProfileName).To(BeNil())
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("NamespacedCloudProfile"))
			})

			It("should remove the cloudProfileName and leave the cloudProfile untouched for an invalid kind (failure is evaluated at another point in the validation chain, fields are only synced here)", func() {
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfileName: ptr.To("profile"), CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile-secret", Kind: "Secret"}}}
				gardenerutils.SyncCloudProfileFields(nil, shoot)
				Expect(shoot.Spec.CloudProfileName).To(BeNil())
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile-secret"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("Secret"))
			})

			It("should remove the cloudProfileName and leave the cloudProfile untouched for an invalid kind with enabled nscpfl feature toggle (failure is evaluated at another point in the validation chain, fields are only synced here)", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfileName: ptr.To("profile"), CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile-secret", Kind: "Secret"}}}
				gardenerutils.SyncCloudProfileFields(nil, shoot)
				Expect(shoot.Spec.CloudProfileName).To(BeNil())
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile-secret"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("Secret"))
			})

			It("should keep changes to the cloudProfile reference if it changes from a NamespacedCloudProfile to a CloudProfile to enable further validations to return an error", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, false))
				oldShoot := &core.Shoot{Spec: core.ShootSpec{CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}}}
				shoot := &core.Shoot{Spec: core.ShootSpec{CloudProfile: &core.CloudProfileReference{Name: "profile", Kind: "CloudProfile"}}}
				gardenerutils.SyncCloudProfileFields(oldShoot, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			It("should keep a NamespacedCloudProfile reference if it has been enabled before", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, false))
				oldShoot := &core.Shoot{Spec: core.ShootSpec{CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}}}
				shoot := oldShoot.DeepCopy()
				gardenerutils.SyncCloudProfileFields(oldShoot, shoot)
				Expect(shoot.Spec.CloudProfileName).To(BeNil())
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("NamespacedCloudProfile"))
			})
		})

		Describe("#SyncArchitectureCapabilityFields", func() {
			var (
				cloudProfileSpecNew core.CloudProfileSpec
				cloudProfileSpecOld core.CloudProfileSpec
			)

			BeforeEach(func() {
				cloudProfileSpecNew = core.CloudProfileSpec{
					MachineImages: []core.MachineImage{
						{Versions: []core.MachineImageVersion{{}}},
					},
					MachineTypes: []core.MachineType{
						{},
					},
				}
				cloudProfileSpecOld = cloudProfileSpecNew
			})

			Describe("Initial migration", func() {
				BeforeEach(func() {
					cloudProfileSpecNew.Capabilities = []core.CapabilityDefinition{
						{Name: "architecture", Values: []string{"arm64", "amd64", "custom"}},
					}
				})

				It("It should do nothing for empty architectures and empty capabilities", func() {
					cloudProfileSpecNewBefore := cloudProfileSpecNew
					// With the update, the old fields are unset:
					cloudProfileSpecOld.MachineImages[0].Versions[0].Architectures = []string{"amd64"}
					cloudProfileSpecOld.MachineTypes[0].Architecture = ptr.To("amd64")

					gardenerutils.SyncArchitectureCapabilityFields(cloudProfileSpecNew, cloudProfileSpecOld)

					Expect(cloudProfileSpecNew).To(Equal(cloudProfileSpecNewBefore))
				})

				It("It should correctly handle split-up machine image version capability architectures", func() {
					cloudProfileSpecNew.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"custom"}}},
						{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
						{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
					}

					gardenerutils.SyncArchitectureCapabilityFields(cloudProfileSpecNew, cloudProfileSpecOld)

					Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64", "arm64", "custom"))
				})

				It("It should sync filled architecture fields to empty capabilities", func() {
					cloudProfileSpecNew.MachineImages[0].Versions[0].Architectures = []string{"amd64", "arm64"}
					cloudProfileSpecNew.MachineTypes[0].Architecture = ptr.To("amd64")

					gardenerutils.SyncArchitectureCapabilityFields(cloudProfileSpecNew, cloudProfileSpecOld)

					Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].Architectures).To(Equal([]string{"amd64", "arm64"}))
					Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].CapabilitySets[0].Capabilities["architecture"]).To(BeEquivalentTo([]string{"amd64"}))
					Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].CapabilitySets[1].Capabilities["architecture"]).To(BeEquivalentTo([]string{"arm64"}))
					Expect(cloudProfileSpecNew.MachineTypes[0].Architecture).To(Equal(ptr.To("amd64")))
					Expect(cloudProfileSpecNew.MachineTypes[0].Capabilities["architecture"]).To(BeEquivalentTo([]string{"amd64"}))
				})
			})
		})

		Describe("#GetCloudProfile", func() {
			var (
				ctx        context.Context
				fakeClient client.Client

				namespaceName              string
				cloudProfileName           string
				namespacedCloudProfileName string

				cloudProfile           *gardencorev1beta1.CloudProfile
				namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile

				shoot *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

				ctx = context.Background()

				namespaceName = "foo"
				cloudProfileName = "profile-1"
				namespacedCloudProfileName = "n-profile-1"

				cloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: cloudProfileName,
					},
				}

				namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      namespacedCloudProfileName,
						Namespace: namespaceName,
					},
				}

				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
					},
					Spec: gardencorev1beta1.ShootSpec{},
				}
			})

			It("returns an error if no CloudProfile can be found", func() {
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				_, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(err).To(MatchError(ContainSubstring("cloudprofiles.core.gardener.cloud \"profile-1\" not found")))
			})

			It("returns CloudProfile if present", func() {
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res).To(Equal(cloudProfile))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the CloudProfile from the cloudProfile reference if present despite cloudProfileName", func() {
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfileName = ptr.To("profile-1")
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res).To(Equal(cloudProfile))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the CloudProfile from the cloudProfile reference with empty kind field", func() {
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Name: cloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res).To(Equal(cloudProfile))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns NamespacedCloudProfile", func() {
				Expect(fakeClient.Create(ctx, namespacedCloudProfile)).To(Succeed())

				shoot.Spec.CloudProfileName = &cloudProfileName
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				}
				res, err := gardenerutils.GetCloudProfile(ctx, fakeClient, shoot)
				Expect(res.Spec).To(Equal(namespacedCloudProfile.Status.CloudProfileSpec))
				Expect(res.Namespace).To(Equal(namespaceName))
				Expect(res.Name).To(Equal(namespacedCloudProfileName))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("ImagesContext", func() {
		Describe("#NewCoreImagesContext", func() {
			It("should successfully construct an ImagesContext from core.MachineImage slice", func() {
				imagesContext := gardenerutils.NewCoreImagesContext([]core.MachineImage{
					{Name: "image-1", Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}},
					}},
					{Name: "image-2", Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "3.0.0"}},
					}},
				})

				i, exists := imagesContext.GetImage("image-1")
				Expect(exists).To(BeTrue())
				Expect(i.Name).To(Equal("image-1"))
				Expect(i.Versions).To(ConsistOf(
					core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}},
					core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}},
				))

				i, exists = imagesContext.GetImage("image-2")
				Expect(exists).To(BeTrue())
				Expect(i.Name).To(Equal("image-2"))
				Expect(i.Versions).To(ConsistOf(
					core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "3.0.0"}},
				))

				i, exists = imagesContext.GetImage("image-99")
				Expect(exists).To(BeFalse())
				Expect(i.Name).To(Equal(""))
				Expect(i.Versions).To(BeEmpty())

				v, exists := imagesContext.GetImageVersion("image-1", "1.0.0")
				Expect(exists).To(BeTrue())
				Expect(v).To(Equal(core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}}))

				v, exists = imagesContext.GetImageVersion("image-1", "99.0.0")
				Expect(exists).To(BeFalse())
				Expect(v).To(Equal(core.MachineImageVersion{}))

				v, exists = imagesContext.GetImageVersion("image-99", "99.0.0")
				Expect(exists).To(BeFalse())
				Expect(v).To(Equal(core.MachineImageVersion{}))
			})
		})

		Describe("#NewV1beta1ImagesContext", func() {
			It("should successfully construct an ImagesContext from v1beta1.MachineImage slice", func() {
				imagesContext := gardenerutils.NewV1beta1ImagesContext([]gardencorev1beta1.MachineImage{
					{Name: "image-1", Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}},
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "2.0.0"}},
					}},
					{Name: "image-2", Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "3.0.0"}},
					}},
				})

				i, exists := imagesContext.GetImage("image-1")
				Expect(exists).To(BeTrue())
				Expect(i.Name).To(Equal("image-1"))
				Expect(i.Versions).To(ConsistOf(
					gardencorev1beta1.MachineImageVersion{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}},
					gardencorev1beta1.MachineImageVersion{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "2.0.0"}},
				))

				i, exists = imagesContext.GetImage("image-2")
				Expect(exists).To(BeTrue())
				Expect(i.Name).To(Equal("image-2"))
				Expect(i.Versions).To(ConsistOf(
					gardencorev1beta1.MachineImageVersion{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "3.0.0"}},
				))

				i, exists = imagesContext.GetImage("image-99")
				Expect(exists).To(BeFalse())
				Expect(i.Name).To(Equal(""))
				Expect(i.Versions).To(BeEmpty())

				v, exists := imagesContext.GetImageVersion("image-1", "1.0.0")
				Expect(exists).To(BeTrue())
				Expect(v).To(Equal(gardencorev1beta1.MachineImageVersion{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}}))

				v, exists = imagesContext.GetImageVersion("image-1", "99.0.0")
				Expect(exists).To(BeFalse())
				Expect(v).To(Equal(gardencorev1beta1.MachineImageVersion{}))

				v, exists = imagesContext.GetImageVersion("image-99", "99.0.0")
				Expect(exists).To(BeFalse())
				Expect(v).To(Equal(gardencorev1beta1.MachineImageVersion{}))
			})
		})
	})
})
