// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/required/virtual"
)

var _ = Describe("Add", func() {
	Describe("Reconciler", func() {
		var (
			ctx        context.Context
			log        logr.Logger
			reconciler *Reconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
			reconciler = &Reconciler{}
		})

		Describe("#MapControllerInstallationToExtension", func() {
			var (
				fakeClient client.Client

				mapperFunc handler.MapFunc
			)

			BeforeEach(func() {
				fakeClient = fake.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
				reconciler.RuntimeClient = fakeClient

				mapperFunc = reconciler.MapControllerInstallationToExtension(log)
			})

			Context("without controller installation", func() {
				It("should not return any request", func() {
					requests := mapperFunc(ctx, nil)
					Expect(requests).To(BeEmpty())
				})
			})

			Context("with controller installation", func() {
				var (
					extensionName          string
					controllerInstallation *gardencorev1beta1.ControllerInstallation
					extension              *operatorv1alpha1.Extension
				)

				BeforeEach(func() {
					extensionName = "provider-local"

					controllerInstallation = &gardencorev1beta1.ControllerInstallation{
						ObjectMeta: metav1.ObjectMeta{
							Name: extensionName + "-123",
						},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{
								Name: extensionName,
							},
						},
					}

					extension = &operatorv1alpha1.Extension{
						ObjectMeta: metav1.ObjectMeta{
							Name: extensionName,
						},
					}

					Expect(fakeClient.Create(ctx, extension)).To(Succeed())
				})

				It("should return expected extension request", func() {
					requests := mapperFunc(ctx, controllerInstallation)
					Expect(requests).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: extensionName}}))
				})

				It("should not return any request if no related extension is found", func() {
					controllerInstallation.Spec.RegistrationRef.Name = controllerInstallation.Spec.RegistrationRef.Name + "-foo"
					requests := mapperFunc(ctx, controllerInstallation)
					Expect(requests).To(BeEmpty())
				})
			})
		})
	})

	Describe("#RequiredConditionChangedPredicate", func() {
		var (
			predicate              predicate.Predicate
			controllerInstallation *gardencorev1beta1.ControllerInstallation

			test = func(objectOld, objectNew client.Object, result bool) {
				ExpectWithOffset(1, predicate.Create(event.CreateEvent{Object: objectNew})).To(BeTrue())
				ExpectWithOffset(1, predicate.Update(event.UpdateEvent{ObjectOld: objectOld, ObjectNew: objectNew})).To(Equal(result))
				ExpectWithOffset(1, predicate.Delete(event.DeleteEvent{Object: objectNew})).To(BeTrue())
				ExpectWithOffset(1, predicate.Generic(event.GenericEvent{Object: objectNew})).To(BeTrue())
			}
		)

		BeforeEach(func() {
			predicate = (&Reconciler{}).RequiredConditionChangedPredicate()
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				Status: gardencorev1beta1.ControllerInstallationStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: "Test", Status: gardencorev1beta1.ConditionFalse},
						{Type: "Required", Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}
		})

		It("should return true if required condition changed", func() {
			controllerInstallationOld := controllerInstallation.DeepCopy()
			controllerInstallationOld.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallationOld.Status.Conditions, gardencorev1beta1.Condition{Type: "Required", Status: gardencorev1beta1.ConditionTrue})

			test(controllerInstallationOld, controllerInstallation, true)
		})

		It("should return true if condition was added with status 'True'", func() {
			controllerInstallationOld := controllerInstallation.DeepCopy()
			controllerInstallationOld.Status.Conditions = v1beta1helper.RemoveConditions(controllerInstallationOld.Status.Conditions, "Required")
			controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallationOld.Status.Conditions, gardencorev1beta1.Condition{Type: "Required", Status: gardencorev1beta1.ConditionTrue})

			test(controllerInstallationOld, controllerInstallation, true)
		})

		It("should return false if required condition is not available", func() {
			controllerInstallation.Status.Conditions = v1beta1helper.RemoveConditions(controllerInstallation.Status.Conditions, "Required")

			test(controllerInstallation.DeepCopy(), controllerInstallation, false)
		})

		It("should return false if required condition is unchanged", func() {
			test(controllerInstallation, controllerInstallation, false)
		})

		It("should return false if condition was added with status 'False'", func() {
			controllerInstallationOld := controllerInstallation.DeepCopy()
			controllerInstallationOld.Status.Conditions = v1beta1helper.RemoveConditions(controllerInstallationOld.Status.Conditions, "Required")
			controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallationOld.Status.Conditions, gardencorev1beta1.Condition{Type: "Required", Status: gardencorev1beta1.ConditionFalse})

			test(controllerInstallationOld, controllerInstallation, false)
		})
	})
})
