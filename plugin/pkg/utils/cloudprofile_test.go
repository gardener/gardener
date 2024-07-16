// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("CloudProfile", func() {
	var (
		coreInformerFactory          gardencoreinformers.SharedInformerFactory
		cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
		namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister

		namespaceName              = "foo"
		cloudProfileName           = "profile-1"
		namespacedCloudProfileName = "n-profile-1"

		cloudProfile           *gardencorev1beta1.CloudProfile
		namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile

		shoot *core.Shoot
	)

	BeforeEach(func() {
		coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		cloudProfileLister = coreInformerFactory.Core().V1beta1().CloudProfiles().Lister()
		namespacedCloudProfileLister = coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Lister()

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
			Spec: core.ShootSpec{
				CloudProfileName: &cloudProfileName,
				CloudProfile: &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				},
			},
		}
	})
	Describe("#GetCloudProfile", func() {
		It("returns an error if neither a CloudProfile nor a NamespacedCloudProfile could be found", func() {
			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("returns CloudProfile if present, derived from cloudProfileName", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(Equal(cloudProfile))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns CloudProfile if present, derived from cloudProfile reference", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(Equal(cloudProfile))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns NamespacedCloudProfile if present", func() {
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			shoot.Spec.CloudProfileName = nil
			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res.Spec).To(Equal(namespacedCloudProfile.Status.CloudProfileSpec))
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not derive a NamespacedCloudProfile from cloudProfileName", func() {
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			shoot.Spec.CloudProfileName = &shoot.Spec.CloudProfile.Name
			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, shoot)
			Expect(res).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#ValidateCloudProfileChanges", func() {
		It("should pass if the CloudProfile did not change", func() {
			newShoot := shoot.DeepCopy()

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass if the NamespacedCloudProfile did not change", func() {
			shoot.Spec.CloudProfileName = nil
			newShoot := shoot.DeepCopy()

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass if the CloudProfile is updated to a direct descendant NamespacedCloudProfile", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			newShoot := shoot.DeepCopy()
			newShoot.Spec.CloudProfileName = nil
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail for a change to a different root CloudProfile", func() {
			anotherCloudProfile := cloudProfile.DeepCopy()
			anotherCloudProfile.ObjectMeta.Name = "anotherCloudProfile"
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(anotherCloudProfile)).To(Succeed())

			newShoot := shoot.DeepCopy()
			newShoot.Spec.CloudProfileName = &anotherCloudProfile.ObjectMeta.Name
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).To(HaveOccurred())
		})

		It("should fail for a change to a different NamespacedCloudProfile", func() {
			anotherNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
			anotherNamespacedCloudProfile.ObjectMeta.Name = "anotherNamespacedCloudProfile"
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(anotherNamespacedCloudProfile)).To(Succeed())

			shoot.Spec.CloudProfileName = nil
			newShoot := shoot.DeepCopy()
			newShoot.Spec.CloudProfile.Name = anotherNamespacedCloudProfile.ObjectMeta.Name
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).To(HaveOccurred())
		})

		It("should fail for a change from a NamespacedCloudProfile to its parent CloudProfile", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			newShoot := shoot.DeepCopy()
			shoot.Spec.CloudProfileName = nil
			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShoot, shoot)
			Expect(err).To(HaveOccurred())
		})
	})
})
