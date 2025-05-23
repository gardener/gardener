// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/extensions/pkg/predicate"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

var _ = Describe("Preconditions", func() {
	Describe("IsInGardenNamespacePredicate", func() {
		var (
			pred predicate.Predicate
			obj  *extensionsv1alpha1.Infrastructure
		)

		BeforeEach(func() {
			pred = IsInGardenNamespacePredicate
			obj = &extensionsv1alpha1.Infrastructure{}
		})

		Describe("#Create, #Update, #Delete, #Generic", func() {
			tests := func(run func(client.Object) bool) {
				It("should return false because obj is nil", func() {
					Expect(run(nil)).To(BeFalse())
				})

				It("should return false because obj is not in garden namespace", func() {
					obj.SetNamespace("foo")
					Expect(run(obj)).To(BeFalse())
				})

				It("should return true because obj is in garden namespace", func() {
					obj.SetNamespace("garden")
					Expect(run(obj)).To(BeTrue())
				})
			}

			tests(func(obj client.Object) bool { return pred.Create(event.CreateEvent{Object: obj}) })
			tests(func(obj client.Object) bool { return pred.Update(event.UpdateEvent{ObjectNew: obj}) })
			tests(func(obj client.Object) bool { return pred.Delete(event.DeleteEvent{Object: obj}) })
			tests(func(obj client.Object) bool { return pred.Generic(event.GenericEvent{Object: obj}) })
		})
	})

	Describe("#ShootNotFailedPredicate", func() {
		var (
			ctrl *gomock.Controller
			ctx  context.Context
			mgr  *mockmanager.MockManager

			fakeClient client.Client
			pred       predicate.Predicate

			obj       *extensionsv1alpha1.Infrastructure
			namespace = "shoot--foo--bar"
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			ctx = context.TODO()

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				Build()

			// Create fake manager
			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetClient().Return(fakeClient)

			pred = ShootNotFailedPredicate(ctx, mgr)

			obj = &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}}
		})

		Describe("#Create, #Update", func() {
			tests := func(run func() bool) {
				It("should return true because shoot has no last operation", func() {
					Expect(fakeClient.Create(ctx, computeClusterWithShoot(
						namespace,
						nil,
						nil,
						&gardencorev1beta1.ShootStatus{},
					))).To(Succeed())

					Expect(run()).To(BeTrue())
				})

				It("should return true because shoot last operation state is not failed", func() {
					Expect(fakeClient.Create(ctx, computeClusterWithShoot(
						namespace,
						nil,
						nil,
						&gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{},
						},
					))).To(Succeed())

					Expect(run()).To(BeTrue())
				})

				It("should return false because shoot is failed", func() {
					Expect(fakeClient.Create(ctx, computeClusterWithShoot(
						namespace,
						nil,
						nil,
						&gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								State: gardencorev1beta1.LastOperationStateFailed,
							},
						},
					))).To(Succeed())

					Expect(run()).To(BeFalse())
				})

				It("should return true if it is not a shoot namespace", func() {
					obj.SetNamespace("foo")
					Expect(run()).To(BeTrue())
				})

				It("should return false if it is a shoot namespace, but cluster is not existing", func() {
					ns := &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: namespace,
							Labels: map[string]string{
								v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
							},
						},
					}
					Expect(fakeClient.Create(ctx, ns)).To(Succeed())
					DeferCleanup(func() {
						Expect(fakeClient.Delete(ctx, ns)).To(Succeed())
					})

					Expect(run()).To(BeFalse())
				})
			}

			tests(func() bool { return pred.Create(event.CreateEvent{Object: obj}) })
			tests(func() bool { return pred.Update(event.UpdateEvent{ObjectNew: obj}) })
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(pred.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
