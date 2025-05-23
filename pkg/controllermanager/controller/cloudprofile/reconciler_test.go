// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Reconciler", func() {
	const finalizerName = "gardener"

	var (
		ctx  = context.TODO()
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		cloudProfileName string
		fakeErr          error
		reconciler       reconcile.Reconciler
		cloudProfile     *gardencorev1beta1.CloudProfile
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		cloudProfileName = "test-cloudprofile"
		fakeErr = errors.New("fake err")
		reconciler = &Reconciler{Client: c, Recorder: &record.FakeRecorder{}}
		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:            cloudProfileName,
				ResourceVersion: "42",
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return nil because object not found", func() {
		c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return err because object reading failed", func() {
		c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).Return(fakeErr)

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).To(MatchError(fakeErr))
	})

	Context("when deletion timestamp not set", func() {
		BeforeEach(func() {
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				*obj = *cloudProfile
				return nil
			})
		})

		It("should ensure the finalizer (error)", func() {
			errToReturn := apierrors.NewNotFound(schema.GroupResource{}, cloudProfileName)

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
				return errToReturn
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(err))
		})

		It("should ensure the finalizer (no error)", func() {
			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when deletion timestamp set", func() {
		BeforeEach(func() {
			now := metav1.Now()
			cloudProfile.DeletionTimestamp = &now
			cloudProfile.Finalizers = []string{finalizerName}

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				*obj = *cloudProfile
				return nil
			})
		})

		It("should do nothing because finalizer is not present", func() {
			cloudProfile.Finalizers = nil

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because Shoot referencing CloudProfile exists", func() {
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfileList{}), gomock.Eq(client.MatchingFields{"spec.parent.name": cloudProfileName})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.NamespacedCloudProfileList, _ ...client.ListOption) error {
				(&gardencorev1beta1.NamespacedCloudProfileList{}).DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: "test-namespace"},
						Spec: gardencorev1beta1.ShootSpec{
							CloudProfileName: &cloudProfileName,
						},
					},
				}}).DeepCopyInto(obj)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("Cannot delete CloudProfile")))
		})

		It("should return an error because NamespacedCloudProfile referencing CloudProfile exists", func() {
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfileList{}), gomock.Eq(client.MatchingFields{"spec.parent.name": cloudProfileName})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.NamespacedCloudProfileList, _ ...client.ListOption) error {
				(&gardencorev1beta1.NamespacedCloudProfileList{Items: []gardencorev1beta1.NamespacedCloudProfile{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespacedprofile", Namespace: "test-namespace"},
						Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
							Parent: gardencorev1beta1.CloudProfileReference{
								Kind: "CloudProfile",
								Name: cloudProfileName,
							},
						},
					},
				}}).DeepCopyInto(obj)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("Cannot delete CloudProfile")))
		})

		It("should remove the finalizer (error)", func() {
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfileList{}), gomock.Eq(client.MatchingFields{"spec.parent.name": cloudProfileName})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.NamespacedCloudProfileList, _ ...client.ListOption) error {
				(&gardencorev1beta1.NamespacedCloudProfileList{}).DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
				return fakeErr
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(fakeErr))
		})

		It("should remove the finalizer (no error)", func() {
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfileList{}), gomock.Eq(client.MatchingFields{"spec.parent.name": cloudProfileName})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.NamespacedCloudProfileList, _ ...client.ListOption) error {
				(&gardencorev1beta1.NamespacedCloudProfileList{}).DeepCopyInto(obj)
				return nil
			})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
