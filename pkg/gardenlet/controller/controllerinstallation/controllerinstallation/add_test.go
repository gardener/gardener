// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
)

var _ = Describe("Add", func() {
	var (
		reconciler             *Reconciler
		controllerInstallation *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		controllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "1",
				Name:            "installation",
			},
		}
	})

	Describe("#ControllerInstallationPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ControllerInstallationPredicate()
		})

		Describe("#Create", func() {
			It("should return false", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return true for periodic cache resyncs", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: controllerInstallation.DeepCopy()})).To(BeTrue())
			})

			It("should return true if deletion timestamp changed", func() {
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.ResourceVersion = "2"
				controllerInstallation.DeletionTimestamp = &metav1.Time{}

				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeTrue())
			})

			It("should return true if deployment ref changed", func() {
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.ResourceVersion = "2"
				controllerInstallation.Spec.DeploymentRef = &corev1.ObjectReference{}

				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeTrue())
			})

			It("should return true if registration ref's resourceVersion changed", func() {
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.ResourceVersion = "2"
				controllerInstallation.Spec.RegistrationRef.ResourceVersion = "foo"

				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeTrue())
			})

			It("should return true if seed ref's resourceVersion changed", func() {
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.ResourceVersion = "2"
				controllerInstallation.Spec.SeedRef.ResourceVersion = "foo"

				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeTrue())
			})

			It("should return false if something else changed", func() {
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.ResourceVersion = "2"
				metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, "foo", "bar")

				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("#HelmTypePredicate", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client
			p          predicate.Predicate

			controllerDeployment *gardencorev1.ControllerDeployment
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			p = reconciler.HelmTypePredicate(ctx, fakeClient)

			controllerDeployment = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deployment",
				},
				Helm: &gardencorev1.HelmControllerDeployment{},
			}
			controllerInstallation.Spec.DeploymentRef = &corev1.ObjectReference{Name: controllerDeployment.Name}
		})

		tests := func(f func(client.Object) bool) {
			It("should return false if the object is no ControllerInstallation", func() {
				Expect(f(controllerDeployment)).To(BeFalse())
			})

			It("should return false if the object has no deployment ref", func() {
				controllerInstallation.Spec.DeploymentRef = nil

				Expect(f(controllerInstallation)).To(BeFalse())
			})

			It("should return false if the referenced deployment does not exist", func() {
				Expect(f(controllerInstallation)).To(BeFalse())
			})

			It("should return false if the deployment ref is not of type helm", func() {
				controllerDeployment.Helm = nil
				Expect(fakeClient.Create(ctx, controllerDeployment)).To(Succeed())

				Expect(f(controllerInstallation)).To(BeFalse())
			})

			It("should return true if the deployment ref is of type helm", func() {
				Expect(fakeClient.Create(ctx, controllerDeployment)).To(Succeed())

				Expect(f(controllerInstallation)).To(BeTrue())
			})
		}

		Describe("#Create", func() {
			tests(func(obj client.Object) bool { return p.Create(event.CreateEvent{Object: obj}) })
		})

		Describe("#Update", func() {
			tests(func(obj client.Object) bool { return p.Update(event.UpdateEvent{ObjectNew: obj}) })
		})

		Describe("#Delete", func() {
			tests(func(obj client.Object) bool { return p.Delete(event.DeleteEvent{Object: obj}) })
		})

		Describe("#Generic", func() {
			tests(func(obj client.Object) bool { return p.Generic(event.GenericEvent{Object: obj}) })
		})
	})
})
