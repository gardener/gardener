// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("CloudProfile", func() {
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
			res, err := admissionutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("returns CloudProfile if present, derived from cloudProfileName", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

			shoot.Spec.CloudProfileName = &cloudProfileName
			res, err := admissionutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(Equal(&cloudProfile.Spec))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns CloudProfile if present, derived from cloudProfile reference", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

			shoot.Spec.CloudProfile = &core.CloudProfileReference{
				Name: cloudProfileName,
			}
			res, err := admissionutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(Equal(&cloudProfile.Spec))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns NamespacedCloudProfile if present", func() {
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			shoot.Spec.CloudProfile = &core.CloudProfileReference{
				Kind: "NamespacedCloudProfile",
				Name: namespacedCloudProfileName,
			}
			res, err := admissionutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(Equal(&namespacedCloudProfile.Status.CloudProfileSpec))
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not derive a NamespacedCloudProfile from cloudProfileName", func() {
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			shoot.Spec.CloudProfileName = &namespacedCloudProfileName
			res, err := admissionutils.GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, shoot)
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

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass if the CloudProfile did not change from cloudProfile to cloudProfile", func() {
			shoot.Spec.CloudProfile = &core.CloudProfileReference{
				Kind: "CloudProfile",
				Name: cloudProfileName,
			}
			newShoot := shoot.DeepCopy()

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass if the CloudProfile did not change from cloudProfile to cloudProfileName", func() {
			newShoot := shoot.DeepCopy()
			shoot.Spec.CloudProfile = &core.CloudProfileReference{
				Kind: "CloudProfile",
				Name: cloudProfileName,
			}
			newShoot.Spec.CloudProfileName = &cloudProfileName

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass if the NamespacedCloudProfile did not change", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

			shoot.Spec.CloudProfile = &core.CloudProfileReference{
				Kind: "NamespacedCloudProfile",
				Name: namespacedCloudProfileName,
			}
			newShoot := shoot.DeepCopy()

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
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
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
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
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
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
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
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
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
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
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
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
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#BuildCloudProfileReference", func() {
		It("should return nil for nil shoot", func() {
			Expect(admissionutils.BuildCloudProfileReference(nil)).To(BeNil())
		})

		It("should build and return cloud profile reference from an existing cloudProfileName", func() {
			Expect(admissionutils.BuildCloudProfileReference(&core.Shoot{Spec: core.ShootSpec{
				CloudProfileName: ptr.To("profile-name"),
			}})).To(Equal(&gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: "profile-name",
			}))
		})

		It("should return an existing cloud profile reference", func() {
			Expect(admissionutils.BuildCloudProfileReference(&core.Shoot{Spec: core.ShootSpec{
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
			Expect(admissionutils.BuildCloudProfileReference(&core.Shoot{Spec: core.ShootSpec{
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
			shoot.Spec.Kubernetes.Version = "v1"
		})

		It("should remove the cloudProfileName and leave the cloudProfile untouched for an invalid kind (failure is evaluated at another point in the validation chain, fields are only synced here)", func() {
			shoot.Spec.CloudProfileName = ptr.To("profile")
			shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "namespacedprofile-secret", Kind: "Secret"}
			admissionutils.SyncCloudProfileFields(nil, shoot)
			Expect(shoot.Spec.CloudProfileName).To(BeNil())
			Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile-secret"))
			Expect(shoot.Spec.CloudProfile.Kind).To(Equal("Secret"))
		})

		It("should remove the cloudProfileName if a NamespacedCloudProfile is given and the feature is enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))
			shoot.Spec.CloudProfileName = ptr.To("profile")
			shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}
			admissionutils.SyncCloudProfileFields(nil, shoot)
			Expect(shoot.Spec.CloudProfileName).To(BeNil())
			Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile"))
			Expect(shoot.Spec.CloudProfile.Kind).To(Equal("NamespacedCloudProfile"))
		})

		It("should remove the cloudProfileName and leave the cloudProfile untouched for an invalid kind with enabled nscpfl feature toggle (failure is evaluated at another point in the validation chain, fields are only synced here)", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))
			shoot.Spec.CloudProfileName = ptr.To("profile")
			shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "namespacedprofile-secret", Kind: "Secret"}
			admissionutils.SyncCloudProfileFields(nil, shoot)
			Expect(shoot.Spec.CloudProfileName).To(BeNil())
			Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile-secret"))
			Expect(shoot.Spec.CloudProfile.Kind).To(Equal("Secret"))
		})

		It("should keep a NamespacedCloudProfile reference if it has been enabled before", func() {
			shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}
			oldShoot := shoot.DeepCopy()
			admissionutils.SyncCloudProfileFields(oldShoot, shoot)
			Expect(shoot.Spec.CloudProfileName).To(BeNil())
			Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile"))
			Expect(shoot.Spec.CloudProfile.Kind).To(Equal("NamespacedCloudProfile"))
		})

		Describe("shoot k8s version < v1.34", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.Version = "v1.33.0"
			})

			It("should default the cloudProfile to the cloudProfileName value", func() {
				shoot.Spec.CloudProfileName = ptr.To("profile")
				admissionutils.SyncCloudProfileFields(nil, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			It("should override the cloudProfileName from the cloudProfile", func() {
				shoot.Spec.CloudProfileName = ptr.To("profile-name")
				shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
				admissionutils.SyncCloudProfileFields(nil, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			It("should override cloudProfile from cloudProfileName as with disabledFeatureToggle reference to NamespacedCloudProfile is ignored", func() {
				shoot.Spec.CloudProfileName = ptr.To("profile")
				shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}
				admissionutils.SyncCloudProfileFields(nil, shoot)
				Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
				Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
			})

			Describe("and shoot k8s version < v1.33", func() {
				BeforeEach(func() {
					shoot.Spec.Kubernetes.Version = "v1.32.3"
				})

				It("should default the cloudProfileName to the cloudProfile value and to kind CloudProfile", func() {
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					admissionutils.SyncCloudProfileFields(nil, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should keep changes to the cloudProfile reference if it changes from a NamespacedCloudProfile to a CloudProfile to enable further validations to return an error", func() {
					oldShoot := &core.Shoot{Spec: core.ShootSpec{CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}}}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile", Kind: "CloudProfile"}
					admissionutils.SyncCloudProfileFields(oldShoot, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})
			})

			Describe("and shoot k8s version >= v1.33.0", func() {
				It("should not default the cloudProfileName to the cloudProfile value but add default kind CloudProfile", func() {
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					admissionutils.SyncCloudProfileFields(nil, shoot)
					Expect(shoot.Spec.CloudProfileName).To(BeNil())
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should keep changes to the cloudProfile reference if it changes from a NamespacedCloudProfile to a CloudProfile to enable further validations to return an error", func() {
					oldShoot := &core.Shoot{Spec: core.ShootSpec{CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}}}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile", Kind: "CloudProfile"}
					admissionutils.SyncCloudProfileFields(oldShoot, shoot)
					Expect(shoot.Spec.CloudProfileName).To(BeNil()) // not defaulted anymore
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})
			})
		})

		Describe("shoot k8s version >= v1.34: drop cloudProfileName on update, keep on create for further validation (leading to error)", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.Version = "v1.34.0"
			})

			Describe("create", func() {
				It("should keep cloudProfileName as the only value", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					admissionutils.SyncCloudProfileFields(nil, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile).To(BeNil())
				})

				It("should keep cloudProfileName besides different cloudProfile", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile-name")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					admissionutils.SyncCloudProfileFields(nil, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile-name"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should keep cloudProfileName besides equal cloudProfile", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					admissionutils.SyncCloudProfileFields(nil, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should not modify cloudProfile and cloudProfileName as with disabledFeatureToggle reference to NamespacedCloudProfile is ignored", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}
					admissionutils.SyncCloudProfileFields(nil, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("namespacedprofile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("NamespacedCloudProfile"))
				})

				It("should not default the cloudProfileName to the cloudProfile value but add default kind CloudProfile", func() {
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					admissionutils.SyncCloudProfileFields(nil, shoot)
					Expect(shoot.Spec.CloudProfileName).To(BeNil())
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should keep changes to the cloudProfile reference if it changes from a NamespacedCloudProfile to a CloudProfile but not default cloudProfileName", func() {
					oldShoot := &core.Shoot{Spec: core.ShootSpec{CloudProfile: &core.CloudProfileReference{Name: "namespacedprofile", Kind: "NamespacedCloudProfile"}}}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile", Kind: "CloudProfile"}
					admissionutils.SyncCloudProfileFields(oldShoot, shoot)
					Expect(shoot.Spec.CloudProfileName).To(BeNil())
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})
			})

			Describe("update", func() {
				It("should keep cloudProfileName as the only value", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					admissionutils.SyncCloudProfileFields(shoot.DeepCopy(), shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile).To(BeNil())
				})

				It("should keep cloudProfileName besides different cloudProfile", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile-name")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					admissionutils.SyncCloudProfileFields(shoot.DeepCopy(), shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile-name"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should keep cloudProfileName besides equal cloudProfile if modified", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfileName = ptr.To("my-profile")
					admissionutils.SyncCloudProfileFields(oldShoot, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should keep cloudProfileName besides equal cloudProfile if added", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfileName = nil
					admissionutils.SyncCloudProfileFields(oldShoot, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should drop cloudProfileName besides equal cloudProfile if unchanged", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile"}
					admissionutils.SyncCloudProfileFields(shoot.DeepCopy(), shoot)
					Expect(shoot.Spec.CloudProfileName).To(BeNil())
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("CloudProfile"))
				})

				It("should keep cloudProfileName besides equally named NamespacedCloudProfile, even if unchanged", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{Name: "profile", Kind: "NamespacedCloudProfile"}
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile.Kind = "NamespacedCloudProfile"
					admissionutils.SyncCloudProfileFields(oldShoot, shoot)
					Expect(*shoot.Spec.CloudProfileName).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Name).To(Equal("profile"))
					Expect(shoot.Spec.CloudProfile.Kind).To(Equal("NamespacedCloudProfile"))
				})
			})
		})
	})
})
