// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagepullsecret_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/imagepullsecret"
)

var _ = Describe("Add", func() {
	var (
		reconciler *Reconciler
		secret     *corev1.Secret
	)

	BeforeEach(func() {
		reconciler = &Reconciler{GardenNamespace: v1beta1constants.GardenNamespace}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: v1beta1constants.GardenNamespace,
				Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
			},
		}
	})

	Describe("#ImagePullSecretPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ImagePullSecretPredicate()
		})

		check := func(f func(obj client.Object) bool) {
			It("should return false because object is not in the garden namespace", func() {
				secret.Namespace = "other-namespace"
				Expect(f(secret)).To(BeFalse())
			})

			It("should return false because object has no labels", func() {
				secret.Labels = nil
				Expect(f(secret)).To(BeFalse())
			})

			It("should return false because object has a different garden role", func() {
				secret.Labels[v1beta1constants.GardenRole] = "some-other-role"
				Expect(f(secret)).To(BeFalse())
			})

			It("should return true for a secret in the garden namespace with the image-pull-secret role", func() {
				Expect(f(secret)).To(BeTrue())
			})
		}

		Describe("#Create", func() {
			check(func(obj client.Object) bool { return p.Create(event.CreateEvent{Object: obj}) })
		})

		Describe("#Delete", func() {
			check(func(obj client.Object) bool { return p.Delete(event.DeleteEvent{Object: obj}) })
		})

		Describe("#Generic", func() {
			check(func(obj client.Object) bool { return p.Generic(event.GenericEvent{Object: obj}) })
		})

		Describe("#Update", func() {
			It("should return false because new object is not in the garden namespace", func() {
				old := secret.DeepCopy()
				secret.Namespace = "other"
				Expect(p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: secret})).To(BeFalse())
			})

			It("should return false because new object has no image-pull-secret role", func() {
				old := secret.DeepCopy()
				secret.Labels[v1beta1constants.GardenRole] = "other-role"
				Expect(p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: secret})).To(BeFalse())
			})

			It("should return false because the secret has not changed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: secret, ObjectNew: secret})).To(BeFalse())
			})

			It("should return true because the secret data changed", func() {
				old := secret.DeepCopy()
				secret.Data = map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)}
				Expect(p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: secret})).To(BeTrue())
			})
		})
	})

	Describe("#MapToAllImagePullSecrets", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			reconciler.Client = fakeClient
		})

		It("should return an empty list when no image pull secrets exist", func() {
			Expect(reconciler.MapToAllImagePullSecrets(logr.Discard())(ctx, nil)).To(BeEmpty())
		})

		It("should return reconcile requests for all image pull secrets", func() {
			s1 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-1",
					Namespace: v1beta1constants.GardenNamespace,
					Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
				},
			}
			s2 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-2",
					Namespace: v1beta1constants.GardenNamespace,
					Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
				},
			}
			Expect(fakeClient.Create(ctx, s1)).To(Succeed())
			Expect(fakeClient.Create(ctx, s2)).To(Succeed())

			requests := reconciler.MapToAllImagePullSecrets(logr.Discard())(ctx, nil)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: s1.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: s2.Name}},
			))
		})

		It("should not return secrets with other roles", func() {
			other := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other",
					Namespace: v1beta1constants.GardenNamespace,
					Labels:    map[string]string{v1beta1constants.GardenRole: "foo"},
				},
			}
			Expect(fakeClient.Create(ctx, other)).To(Succeed())

			Expect(reconciler.MapToAllImagePullSecrets(logr.Discard())(ctx, nil)).To(BeEmpty())
		})
	})
})
