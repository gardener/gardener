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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("CloudProfile", func() {
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
