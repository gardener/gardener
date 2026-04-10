// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("cleaner", func() {

	Describe("#cleanupStatefulSet", func() {
		var (
			s          *runtime.Scheme
			ctx        context.Context
			fakeClient client.Client
			sts        *appsv1.StatefulSet
		)

		BeforeEach(func() {
			s = runtime.NewScheme()
			Expect(appsv1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")
			Expect(corev1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")

			ctx = context.TODO()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()

			sts = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo-ns",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Replicas: ptr.To[int32](1),
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								VolumeName:       "foo-pvc",
								StorageClassName: ptr.To("ultra-fast"),
							},
						},
					},
				},
			}
		})

		It("should do nothing if deletePVCs is false", func() {
			err := cleanupStatefulSet(ctx, fakeClient, s, sts, false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should do nothing if conversion to appsv1.StatefulSet fails", func() {
			s = runtime.NewScheme()

			err := cleanupStatefulSet(ctx, fakeClient, s, sts, true)
			Expect(err).To(MatchError(ContainSubstring("failed cleaning up PersistentVolumeClaims of StatefulSet: could not convert object to StatefulSet")))
		})

		It("should do nothing if .spec.volumeClaimTemplate is not set", func() {
			sts.Spec.VolumeClaimTemplates = nil

			err := cleanupStatefulSet(ctx, fakeClient, s, sts, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should do nothing if list PVCs fails", func() {
			fakeErr := errors.New("fake")
			c := fakeclient.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
					return fakeErr
				},
			}).Build()

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).To(MatchError(ContainSubstring(fakeErr.Error())))
		})

		It("should do nothing if all PVCs of the StatefulSet have already been deleted", func() {
			err := cleanupStatefulSet(ctx, fakeClient, s, sts, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete all PVCs of the StatefulSet", func() {
			fakeErr := errors.New("fake")
			c := fakeclient.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
				DeleteAllOf: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteAllOfOption) error {
					return fakeErr
				},
			}).Build()

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-pvc-foo-0",
					Namespace: sts.Namespace,
					Labels:    sts.Spec.Selector.MatchLabels,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					VolumeName:       "foo-pvc-foo-0",
					StorageClassName: ptr.To("ultra-fast"),
				},
			}
			Expect(c.Create(ctx, pvc)).To(Succeed())

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).To(MatchError(ContainSubstring(fakeErr.Error())))
		})

		It("should delete all PVCs of the StatefulSet", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-pvc-foo-0",
					Namespace: sts.Namespace,
					Labels:    sts.Spec.Selector.MatchLabels,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					VolumeName:       "foo-pvc-foo-0",
					StorageClassName: ptr.To("ultra-fast"),
				},
			}
			Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

			err := cleanupStatefulSet(ctx, fakeClient, s, sts, true)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
