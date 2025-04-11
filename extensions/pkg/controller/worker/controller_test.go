// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Controller", func() {
	Describe("#MachineConditionChangedPredicate", func() {
		var (
			ctx context.Context
			c   client.Client

			machineDeployment *machinev1alpha1.MachineDeployment
			oldMachine        *machinev1alpha1.Machine
			machine           *machinev1alpha1.Machine

			p predicate.Predicate
		)

		BeforeEach(func() {
			ctx = context.Background()
			c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			machineDeployment = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machineDeployment",
					Namespace: "namespace",
				},
				Spec: machinev1alpha1.MachineDeploymentSpec{
					Strategy: machinev1alpha1.MachineDeploymentStrategy{
						Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
						InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
							OrchestrationType: machinev1alpha1.OrchestrationTypeManual,
						},
					},
				},
				Status: machinev1alpha1.MachineDeploymentStatus{
					Replicas:        2,
					UpdatedReplicas: 2,
				},
			}

			machine = &machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine",
					Namespace: "namespace",
					Labels: map[string]string{
						"name": machineDeployment.Name,
					},
				},
				Status: machinev1alpha1.MachineStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   machinev1alpha1.NodeInPlaceUpdate,
							Status: corev1.ConditionTrue,
							Reason: machinev1alpha1.CandidateForUpdate,
						},
					},
				},
			}

			Expect(c.Create(ctx, machineDeployment)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(c.Delete(ctx, machineDeployment))).To(Succeed())
			})

			oldMachine = machine.DeepCopy()
			p = MachineConditionChangedPredicate(ctx, logr.Discard(), c)
		})

		Describe("#Create", func() {
			It("should return false when object is not machine", func() {
				Expect(p.Create(event.CreateEvent{Object: &corev1.Secret{}})).To(BeFalse())
			})

			It("should return false when machine does not have a machineDeployment label", func() {
				delete(machine.Labels, "name")
				Expect(p.Create(event.CreateEvent{Object: machine})).To(BeFalse())
			})

			It("should return false when machineDeployment is not found", func() {
				Expect(c.Delete(ctx, machineDeployment)).To(Succeed())
				Expect(p.Create(event.CreateEvent{Object: machine})).To(BeFalse())
			})

			It("should return false when machineDeployment strategy type is not InPlace", func() {
				machineDeployment.Spec.Strategy.Type = machinev1alpha1.RollingUpdateMachineDeploymentStrategyType
				Expect(c.Update(ctx, machineDeployment)).To(Succeed())

				Expect(p.Create(event.CreateEvent{Object: machine})).To(BeFalse())
			})

			It("should return false when machineDeployment orchestration type is not manual", func() {
				machineDeployment.Spec.Strategy.InPlaceUpdate.OrchestrationType = machinev1alpha1.OrchestrationTypeAuto
				Expect(c.Update(ctx, machineDeployment)).To(Succeed())

				Expect(p.Create(event.CreateEvent{Object: machine})).To(BeFalse())
			})

			It("should return true when machineDeployment strategy type is InPlace and orchestration type is Manual", func() {
				Expect(p.Create(event.CreateEvent{Object: machine})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is not machine", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: &corev1.Secret{},
				})).To(BeFalse())
			})

			It("should return false because old object is not machine", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: &corev1.Secret{},
					ObjectNew: machine,
				})).To(BeFalse())
			})

			It("should return false because the machine strategy type is not InPlace", func() {
				machineDeployment.Spec.Strategy.Type = machinev1alpha1.RollingUpdateMachineDeploymentStrategyType
				Expect(c.Update(ctx, machineDeployment)).To(Succeed())

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: machine,
				})).To(BeFalse())
			})

			It("should return false because the machineDeployment orchestration type is not manual", func() {
				machineDeployment.Spec.Strategy.InPlaceUpdate.OrchestrationType = machinev1alpha1.OrchestrationTypeAuto
				Expect(c.Update(ctx, machineDeployment)).To(Succeed())

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: machine,
				})).To(BeFalse())
			})

			It("should return false if oldMachine InPlaceUpdate condition is nil or newMachine InPlaceUpdate condition is nil", func() {
				oldMachine.Status.Conditions = nil
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: machine,
				})).To(BeFalse())

				machine.Status.Conditions = nil
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: machine,
				})).To(BeFalse())
			})

			It("should return false if oldMachine InPlaceUpdate condition reason is not UpdateCandidate or newMachine InPlaceUpdate condition reason is not SelectedForUpdate", func() {
				oldMachine.Status.Conditions[0].Reason = "notUpdateCandidate"
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: machine,
				})).To(BeFalse())

				machine.Status.Conditions[0].Reason = "notSelectedForUpdate"
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: machine,
				})).To(BeFalse())
			})

			It("should return true if oldMachine InPlaceUpdate condition reason is UpdateCandidate and newMachine InPlaceUpdate condition reason is SelectedForUpdate", func() {
				machine.Status.Conditions[0].Reason = machinev1alpha1.SelectedForUpdate

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldMachine,
					ObjectNew: machine,
				})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
