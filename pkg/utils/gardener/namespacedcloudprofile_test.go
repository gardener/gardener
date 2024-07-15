// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("NamespacedCloudProfile", func() {
	Describe("#GetCloudProfile", func() {

		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")

			namespaceName    = "foo"
			cloudProfileName = "profile-1"

			cloudProfile           *gardencorev1beta1.CloudProfile
			namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
			}

			namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cloudProfileName,
					Namespace: namespaceName,
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("returns an error if neither a CloudProfile nor a NamespacedCloudProfile could be found", func() {
			c.EXPECT().Get(ctx, pkgclient.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).Return(fakeErr)

			res, err := gardenerutils.GetCloudProfile(ctx, c, gardenerutils.BuildCloudProfileReference(&cloudProfileName, nil), namespaceName)
			Expect(res).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		It("returns CloudProfile if present", func() {
			c.EXPECT().Get(ctx,
				pkgclient.ObjectKey{Name: cloudProfileName},
				gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}),
			).DoAndReturn(func(_ context.Context, _ pkgclient.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...pkgclient.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			res, err := gardenerutils.GetCloudProfile(ctx, c, gardenerutils.BuildCloudProfileReference(&cloudProfileName, nil), namespaceName)
			Expect(res).To(Equal(cloudProfile))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the CloudProfile referenced in the CloudProfileName if present, taking precedence over cloudProfile reference", func() {
			c.EXPECT().Get(ctx,
				pkgclient.ObjectKey{Name: cloudProfileName},
				gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}),
			).DoAndReturn(func(_ context.Context, _ pkgclient.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...pkgclient.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			res, err := gardenerutils.GetCloudProfile(ctx, c, gardenerutils.BuildCloudProfileReferenceV1Beta1(&cloudProfileName, &gardencorev1beta1.CloudProfileReference{
				Kind: "NamespacedCloudProfile",
				Name: cloudProfileName + "-dont-care",
			}), namespaceName)
			Expect(res).To(Equal(cloudProfile))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns NamespacedCloudProfile if present", func() {
			c.EXPECT().Get(ctx,
				pkgclient.ObjectKey{Name: cloudProfileName, Namespace: namespaceName},
				gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}),
			).DoAndReturn(func(_ context.Context, _ pkgclient.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...pkgclient.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			res, err := gardenerutils.GetCloudProfile(ctx, c, gardenerutils.BuildCloudProfileReferenceV1Beta1(nil, &gardencorev1beta1.CloudProfileReference{
				Kind: "NamespacedCloudProfile",
				Name: cloudProfileName,
			}), namespaceName)
			Expect(res.Spec).To(Equal(namespacedCloudProfile.Status.CloudProfileSpec))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
