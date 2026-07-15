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
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed/imagepullsecret"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Add", func() {
	const seedName = "test-seed"

	var (
		reconciler    *Reconciler
		seedNamespace string
	)

	BeforeEach(func() {
		seedNamespace = gardenerutils.ComputeGardenNamespace(seedName)
		reconciler = &Reconciler{SeedName: seedName}
	})

	Describe("#ImagePullSecretPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ImagePullSecretPredicate(seedNamespace)
		})

		check := func(f func(obj client.Object) bool) {
			It("should return false because object is not in the seed-scoped namespace", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "other-namespace",
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
					},
				}
				Expect(f(secret)).To(BeFalse())
			})

			It("should return false because object has no image-pull-secret role", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: seedNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: "other-role"},
					},
				}
				Expect(f(secret)).To(BeFalse())
			})

			It("should return false because object has no labels", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Namespace: seedNamespace},
				}
				Expect(f(secret)).To(BeFalse())
			})

			It("should return true for a secret in the seed-scoped namespace with the image-pull-secret role", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: seedNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
					},
				}
				Expect(f(secret)).To(BeTrue())
			})
		}

		Describe("#Create", func() {
			check(func(obj client.Object) bool { return p.Create(event.CreateEvent{Object: obj}) })
		})

		Describe("#Update", func() {
			check(func(obj client.Object) bool { return p.Update(event.UpdateEvent{ObjectNew: obj}) })
		})

		Describe("#Delete", func() {
			check(func(obj client.Object) bool { return p.Delete(event.DeleteEvent{Object: obj}) })
		})

		Describe("#Generic", func() {
			check(func(obj client.Object) bool { return p.Generic(event.GenericEvent{Object: obj}) })
		})
	})

	Describe("#TargetNamespacePredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.TargetNamespacePredicate()
		})

		check := func(f func(obj client.Object) bool) {
			It("should return false because object has no relevant role", func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1constants.GardenRole: "seed"}},
				}
				Expect(f(ns)).To(BeFalse())
			})

			It("should return false because object has no labels", func() {
				Expect(f(&corev1.Namespace{})).To(BeFalse())
			})

			It("should return true for an extension namespace", func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}},
				}
				Expect(f(ns)).To(BeTrue())
			})

			It("should return true for a shoot control plane namespace", func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}},
				}
				Expect(f(ns)).To(BeTrue())
			})
		}

		Describe("#Create", func() {
			check(func(obj client.Object) bool { return p.Create(event.CreateEvent{Object: obj}) })
		})

		Describe("#Update", func() {
			check(func(obj client.Object) bool { return p.Update(event.UpdateEvent{ObjectNew: obj}) })
		})

		Describe("#Delete", func() {
			check(func(obj client.Object) bool { return p.Delete(event.DeleteEvent{Object: obj}) })
		})

		Describe("#Generic", func() {
			check(func(obj client.Object) bool { return p.Generic(event.GenericEvent{Object: obj}) })
		})
	})

	Describe("#MapToAllImagePullSecrets", func() {
		var (
			ctx        = context.TODO()
			seedClient client.Client
		)

		BeforeEach(func() {
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			reconciler.SeedClient = seedClient
		})

		It("should return an empty list when no image pull secrets exist", func() {
			Expect(reconciler.MapToAllImagePullSecrets(logr.Discard())(ctx, nil)).To(BeEmpty())
		})

		It("should return reconcile requests for all image pull secrets in the garden namespace", func() {
			secret1 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret-1",
					Namespace: v1beta1constants.GardenNamespace,
					Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
				},
			}
			secret2 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret-2",
					Namespace: v1beta1constants.GardenNamespace,
					Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
				},
			}
			Expect(seedClient.Create(ctx, secret1)).To(Succeed())
			Expect(seedClient.Create(ctx, secret2)).To(Succeed())

			requests := reconciler.MapToAllImagePullSecrets(logr.Discard())(ctx, nil)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: secret1.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: secret2.Name}},
			))
		})

		It("should not return secrets with other roles", func() {
			other := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-secret",
					Namespace: v1beta1constants.GardenNamespace,
					Labels:    map[string]string{v1beta1constants.GardenRole: "some-other-role"},
				},
			}
			Expect(seedClient.Create(ctx, other)).To(Succeed())

			Expect(reconciler.MapToAllImagePullSecrets(logr.Discard())(ctx, nil)).To(BeEmpty())
		})
	})
})
