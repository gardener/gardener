// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Machines", func() {
	machineSetName := "machine-set-1"
	machineDeploymentName := "machine-deployment-1"
	var (
		machineSetReference = machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineSetKind,
						Name: machineSetName,
					},
				},
			},
		}

		machineDeploymentReference = machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineDeploymentKind,
						Name: machineDeploymentName,
					},
				},
			},
		}

		machineLabelReference = machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"name": machineDeploymentName,
				},
			},
		}
	)

	DescribeTable("#BuildOwnerToMachinesMap", func(machines []machinev1alpha1.Machine, expected map[string][]machinev1alpha1.Machine) {
		result := BuildOwnerToMachinesMap(machines)
		Expect(result).To(Equal(expected))
	},
		Entry("should map using reference kind = `MachineSet`", []machinev1alpha1.Machine{machineSetReference, machineLabelReference}, map[string][]machinev1alpha1.Machine{
			machineSetName: {machineSetReference}, machineDeploymentName: {machineLabelReference},
		}),

		Entry("should map using label with key `name`", []machinev1alpha1.Machine{machineLabelReference}, map[string][]machinev1alpha1.Machine{
			machineDeploymentName: {machineLabelReference},
		}),

		Entry("should not consider machines with machine deployment reference", []machinev1alpha1.Machine{machineSetReference, machineDeploymentReference, machineLabelReference}, map[string][]machinev1alpha1.Machine{
			machineSetName: {machineSetReference}, machineDeploymentName: {machineLabelReference},
		}),
	)

	var (
		machineSetWithOwnerReference = machinev1alpha1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineDeploymentKind,
						Name: machineDeploymentName,
					},
				},
			},
		}

		machineSetWithWrongOwnerReference = machinev1alpha1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineSetKind,
						Name: machineDeploymentName,
					},
				},
			},
		}

		machineSetWithLabelReference = machinev1alpha1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"name": machineDeploymentName,
				},
			},
		}
	)

	DescribeTable("#BuildOwnerToMachineSetsMap", func(machines []machinev1alpha1.MachineSet, expected map[string][]machinev1alpha1.MachineSet) {
		result := BuildOwnerToMachineSetsMap(machines)
		Expect(result).To(Equal(expected))
	},
		Entry("should map using reference kind = `MachineDeployment", []machinev1alpha1.MachineSet{machineSetWithOwnerReference}, map[string][]machinev1alpha1.MachineSet{
			machineDeploymentName: {machineSetWithOwnerReference},
		}),

		Entry("should map using label with key `name`", []machinev1alpha1.MachineSet{machineSetWithLabelReference}, map[string][]machinev1alpha1.MachineSet{
			machineDeploymentName: {machineSetWithLabelReference},
		}),

		Entry("should not consider machines with machine set reference", []machinev1alpha1.MachineSet{machineSetWithOwnerReference, machineSetWithLabelReference, machineSetWithWrongOwnerReference}, map[string][]machinev1alpha1.MachineSet{
			machineDeploymentName: {machineSetWithOwnerReference, machineSetWithLabelReference},
		}),
	)

	DescribeTable("#BuildMachineSetToMachinesMap", func(machines []machinev1alpha1.Machine, expected map[string][]machinev1alpha1.Machine) {
		result := BuildMachineSetToMachinesMap(machines)
		Expect(result).To(Equal(expected))
	},
		Entry("should map using reference kind = `MachineSet`", []machinev1alpha1.Machine{machineSetReference}, map[string][]machinev1alpha1.Machine{
			machineSetName: {machineSetReference},
		}),

		Entry("should not map if reference kind is not `MachineSet`", []machinev1alpha1.Machine{machineDeploymentReference}, map[string][]machinev1alpha1.Machine{}),

		Entry("should map multiple machines to the same MachineSet", []machinev1alpha1.Machine{machineSetReference, machineSetReference}, map[string][]machinev1alpha1.Machine{
			machineSetName: {machineSetReference, machineSetReference},
		}),

		Entry("should not map machines without owner references", []machinev1alpha1.Machine{machineLabelReference}, map[string][]machinev1alpha1.Machine{}),
	)

	Describe("#WaitUntilMachineResourcesDeleted", func() {
		var (
			ctx        = context.TODO()
			log        = logr.Discard()
			fakeClient client.Client
			fakeOps    *retryfake.Ops

			namespace = "namespace"
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		It("should succeed because no machine objects exist", func() {
			Expect(WaitUntilMachineResourcesDeleted(ctx, log, fakeClient, namespace)).To(Succeed())
		})

		It("should fail because MachineDeployments are left", func() {
			Expect(fakeClient.Create(ctx, &machinev1alpha1.MachineDeployment{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: namespace}})).To(Succeed())
			Expect(WaitUntilMachineResourcesDeleted(ctx, log, fakeClient, namespace)).To(MatchError(ContainSubstring("waiting until the following machine resources have been deleted: 0 machines, 0 machine sets, 1 machine deployments, 0 machine classes, 0 machine class secrets")))
		})

		It("should fail because MachineSets are left", func() {
			Expect(fakeClient.Create(ctx, &machinev1alpha1.MachineSet{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: namespace}})).To(Succeed())
			Expect(WaitUntilMachineResourcesDeleted(ctx, log, fakeClient, namespace)).To(MatchError(ContainSubstring("waiting until the following machine resources have been deleted: 0 machines, 1 machine sets, 0 machine deployments, 0 machine classes, 0 machine class secrets")))
		})

		It("should fail because Machine are left", func() {
			Expect(fakeClient.Create(ctx, &machinev1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: namespace}})).To(Succeed())
			Expect(WaitUntilMachineResourcesDeleted(ctx, log, fakeClient, namespace)).To(MatchError(ContainSubstring("waiting until the following machine resources have been deleted: 1 machines, 0 machine sets, 0 machine deployments, 0 machine classes, 0 machine class secrets")))
		})

		It("should fail because MachineClasses are left", func() {
			Expect(fakeClient.Create(ctx, &machinev1alpha1.MachineClass{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: namespace}})).To(Succeed())
			Expect(WaitUntilMachineResourcesDeleted(ctx, log, fakeClient, namespace)).To(MatchError(ContainSubstring("waiting until the following machine resources have been deleted: 0 machines, 0 machine sets, 0 machine deployments, 1 machine classes, 0 machine class secrets")))
		})

		It("should fail because MachineClass secrets are left", func() {
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: namespace, Labels: map[string]string{"gardener.cloud/purpose": "machineclass"}, Finalizers: []string{"foo"}}})).To(Succeed())
			Expect(WaitUntilMachineResourcesDeleted(ctx, log, fakeClient, namespace)).To(MatchError(ContainSubstring("waiting until the following machine resources have been deleted: 0 machines, 0 machine sets, 0 machine deployments, 0 machine classes, 1 machine class secrets")))
		})
	})

	DescribeTable("#IsMachineDeploymentStrategyManualInPlace", func(strategy machinev1alpha1.MachineDeploymentStrategy, expected bool) {
		Expect(IsMachineDeploymentStrategyManualInPlace(strategy)).To(Equal(expected))
	},
		Entry("strategy type is not InPlaceUpdate",
			machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
			},
			false,
		),

		Entry("strategy type is InPlaceUpdate but InPlaceUpdate field is nil",
			machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
			},
			false,
		),

		Entry("strategy type is InPlaceUpdate but orchestration type is not Manual",
			machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
					OrchestrationType: machinev1alpha1.OrchestrationTypeAuto,
				},
			},
			false,
		),

		Entry("strategy type is InPlaceUpdate and orchestration type is Manual",
			machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
					OrchestrationType: machinev1alpha1.OrchestrationTypeManual,
				},
			},
			true,
		),
	)
})
