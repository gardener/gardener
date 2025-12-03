// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/project"
)

var _ = Describe("Add", func() {
	var reconciler *project.Reconciler

	BeforeEach(func() {
		reconciler = &project.Reconciler{}
	})

	Describe("#RoleBindingPredicate", func() {
		var (
			p           predicate.Predicate
			roleBinding *rbacv1.RoleBinding
		)

		BeforeEach(func() {
			p = reconciler.RoleBindingPredicate()
			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "1",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     "project-member",
				},
				Subjects: []rbacv1.Subject{{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     "admin",
				}},
			}
		})

		Describe("#Create", func() {
			It("should return false", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return true for periodic cache resyncs", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: roleBinding.DeepCopy()})).To(BeTrue())
			})

			It("should return true if roleRef changed", func() {
				oldRoleBinding := roleBinding.DeepCopy()
				roleBinding.ResourceVersion = "2"
				roleBinding.RoleRef.Name = "other"

				Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: oldRoleBinding})).To(BeTrue())
			})

			It("should return true if subjects changed", func() {
				oldRoleBinding := roleBinding.DeepCopy()
				roleBinding.ResourceVersion = "2"
				roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     "viewer",
				})

				Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: oldRoleBinding})).To(BeTrue())
			})

			It("should return false if something else changed", func() {
				oldRoleBinding := roleBinding.DeepCopy()
				roleBinding.ResourceVersion = "2"
				metav1.SetMetaDataLabel(&roleBinding.ObjectMeta, "foo", "bar")

				Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: oldRoleBinding})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})

	Describe("#MapShootToProjectInDeletion", func() {
		var (
			ctx        = context.Background()
			log        logr.Logger
			fakeClient client.Client
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				Build()
			reconciler.Client = fakeClient
		})

		It("should do nothing if the object is no shoot", func() {
			Expect(reconciler.MapShootToProjectInDeletion(log)(ctx, &gardencorev1beta1.Project{})).To(BeEmpty())
		})

		When("other shoots exist in namespace", func() {
			var (
				namespace = "garden-test"

				shoot         *metav1.PartialObjectMetadata
				existingShoot *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				shoot = &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
						Kind:       "Shoot",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-shoot",
						Namespace: namespace,
					},
				}
				existingShoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-shoot",
						Namespace: namespace,
					},
				}
			})

			It("should return empty list of projects", func() {
				Expect(fakeClient.Create(ctx, existingShoot)).To(Succeed())
				Expect(reconciler.MapShootToProjectInDeletion(log)(ctx, shoot)).To(BeEmpty())
			})
		})

		When("no other shoots exist", func() {
			var (
				namespace   = "garden-test"
				projectName = "test-project"

				shoot            *metav1.PartialObjectMetadata
				project          *gardencorev1beta1.Project
				projectNamespace *corev1.Namespace
			)

			BeforeEach(func() {
				shoot = &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
						Kind:       "Shoot",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-shoot",
						Namespace: namespace,
					},
				}

				project = &gardencorev1beta1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name:       projectName,
						Finalizers: []string{"gardener"},
					},
					Spec: gardencorev1beta1.ProjectSpec{
						Namespace: &namespace,
					},
				}

				projectNamespace = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
						Labels: map[string]string{
							"project.gardener.cloud/name": projectName,
						},
					},
				}

				Expect(fakeClient.Create(ctx, project)).To(Succeed())
				Expect(fakeClient.Create(ctx, projectNamespace)).To(Succeed())
			})

			It("should map the shoot to project when project is being deleted", func() {
				Expect(fakeClient.Delete(ctx, project)).To(Succeed())

				Expect(reconciler.MapShootToProjectInDeletion(log)(ctx, shoot)).To(Equal([]reconcile.Request{
					{NamespacedName: types.NamespacedName{Name: projectName}},
				}))
			})

			It("should return empty list of projects if project is not being deleted", func() {
				Expect(reconciler.MapShootToProjectInDeletion(log)(ctx, shoot)).To(BeEmpty())
			})
		})
	})
})
