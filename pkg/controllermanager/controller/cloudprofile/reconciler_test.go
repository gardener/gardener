// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	const finalizerName = "gardener"

	var (
		ctx        = context.TODO()
		fakeClient client.Client

		cloudProfileName string
		fakeErr          error
		reconciler       reconcile.Reconciler
		cloudProfile     *gardencorev1beta1.CloudProfile
	)

	BeforeEach(func() {
		cloudProfileName = "test-cloudprofile"
		fakeErr = errors.New("fake err")

		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(
				&gardencorev1beta1.NamespacedCloudProfile{},
				core.NamespacedCloudProfileParentRefName,
				indexer.NamespacedCloudProfileParentRefNameIndexerFunc,
			).
			Build()
		reconciler = &Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}
		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: cloudProfileName,
			},
		}
	})

	It("should return nil because object not found", func() {
		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return err because object reading failed", func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return fakeErr
				},
			}).
			Build()
		reconciler = &Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).To(MatchError(fakeErr))
	})

	Context("when deletion timestamp not set", func() {
		BeforeEach(func() {
			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
		})

		It("should ensure the finalizer (error)", func() {
			patchCalls := 0
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						patchCalls++
						return fakeErr
					},
				}).
				Build()
			reconciler = &Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(HaveOccurred())
			Expect(patchCalls).To(Equal(1))
		})

		It("should ensure the finalizer (no error)", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, cloudProfile)).To(Succeed())
			Expect(cloudProfile.Finalizers).To(ContainElement(finalizerName))
		})
	})

	Context("when deletion timestamp set", func() {
		BeforeEach(func() {
			cloudProfile.Finalizers = []string{finalizerName}
			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Delete(ctx, cloudProfile.DeepCopy())).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, cloudProfile)).To(Succeed())
		})

		It("should do nothing because finalizer is not present", func() {
			cloudProfile.Finalizers = nil
			Expect(fakeClient.Patch(ctx, cloudProfile, client.MergeFrom(cloudProfile))).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because Shoot referencing CloudProfile exists", func() {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: "test-namespace"},
				Spec: gardencorev1beta1.ShootSpec{
					CloudProfileName: &cloudProfileName,
				},
			}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("Cannot delete CloudProfile")))
		})

		It("should return an error because NamespacedCloudProfile referencing CloudProfile exists", func() {
			ncpProfile := &gardencorev1beta1.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{Name: "test-namespacedprofile", Namespace: "test-namespace"},
				Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
					Parent: gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: cloudProfileName,
					},
				},
			}
			Expect(fakeClient.Create(ctx, ncpProfile)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("Cannot delete CloudProfile")))
		})

		It("should remove the finalizer (error)", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithIndex(
					&gardencorev1beta1.NamespacedCloudProfile{},
					core.NamespacedCloudProfileParentRefName,
					indexer.NamespacedCloudProfileParentRefNameIndexerFunc,
				).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return fakeErr
					},
				}).
				Build()
			reconciler = &Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

			cp := cloudProfile.DeepCopy()
			cp.ResourceVersion = ""
			cp.Finalizers = []string{finalizerName}
			Expect(fakeClient.Create(ctx, cp)).To(Succeed())
			Expect(fakeClient.Delete(ctx, cp.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(fakeErr))
		})

		It("should remove the finalizer (no error)", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cloudProfileName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, &gardencorev1beta1.CloudProfile{})).To(BeNotFoundError())
		})
	})
})
