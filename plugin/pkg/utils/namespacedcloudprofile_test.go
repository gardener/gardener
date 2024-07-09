// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("NamespacedCloudProfile", func() {
	var (
		coreInformerFactory          gardencoreinformers.SharedInformerFactory
		cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
		namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister

		namespaceName              = "foo"
		cloudProfileName           = "profile-1"
		namespacedCloudProfileName = "n-profile-1"

		cloudProfile           *gardencorev1beta1.CloudProfile
		namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
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
	})
	Describe("#GetCloudProfile", func() {
		It("returns an error if neither a CloudProfile nor a NamespacedCloudProfile could be found", func() {
			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, nil, &cloudProfileName, namespaceName)
			Expect(res).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("returns CloudProfile if present, derived from cloudProfileName", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, nil, &cloudProfileName, namespaceName)
			Expect(res).To(Equal(cloudProfile))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns CloudProfile if present, derived from cloudProfile reference", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())

			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, &core.CloudProfileReference{
				Kind: "CloudProfile",
				Name: cloudProfileName,
			}, nil, namespaceName)
			Expect(res).To(Equal(cloudProfile))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns NamespacedCloudProfile if present", func() {
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, &core.CloudProfileReference{
				Kind: "NamespacedCloudProfile",
				Name: namespacedCloudProfileName,
			}, nil, namespaceName)
			Expect(res.Spec).To(Equal(namespacedCloudProfile.Status.CloudProfileSpec))
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not derive a NamespacedCloudProfile from cloudProfileName", func() {
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			res, err := admissionutils.GetCloudProfile(cloudProfileLister, namespacedCloudProfileLister, nil, &namespacedCloudProfileName, namespaceName)
			Expect(res).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#GetRootCloudProfile", func() {
		It("should correctly determine end at a root CloudProfile", func() {
			cloudProfileReference := &gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: cloudProfileName,
			}

			res, err := admissionutils.GetRootCloudProfile(cloudProfileLister, namespacedCloudProfileLister, cloudProfileReference, namespaceName)
			Expect(res).To(Equal(cloudProfileReference))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should correctly determine a root CloudProfile from a NamespacedCloudProfile", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			cloudProfileReference := &gardencorev1beta1.CloudProfileReference{
				Kind: "NamespacedCloudProfile",
				Name: namespacedCloudProfileName,
			}

			res, err := admissionutils.GetRootCloudProfile(cloudProfileLister, namespacedCloudProfileLister, cloudProfileReference, namespaceName)
			Expect(res).To(Equal(&namespacedCloudProfile.Spec.Parent))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ValidateCloudProfileChanges", func() {
		It("should pass if the CloudProfile did not change", func() {
			oldShootSpec := core.ShootSpec{
				CloudProfileName: &cloudProfileName,
			}
			newShootSpec := *oldShootSpec.DeepCopy()

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShootSpec, oldShootSpec, namespaceName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass if the NamespacedCloudProfile did not change", func() {
			oldShootSpec := core.ShootSpec{
				CloudProfile: &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				},
			}
			newShootSpec := *oldShootSpec.DeepCopy()

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShootSpec, oldShootSpec, namespaceName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass if the determined root cloudprofiles match", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(cloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(namespacedCloudProfile)).To(Succeed())

			oldShootSpec := core.ShootSpec{
				CloudProfile: &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				},
			}
			newShootSpec := core.ShootSpec{
				CloudProfile: &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfileName,
				},
			}

			err := admissionutils.ValidateCloudProfileChanges(cloudProfileLister, namespacedCloudProfileLister, newShootSpec, oldShootSpec, namespaceName)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
