// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("MachineState", func() {
	Describe("MarshalMachineState and UnmarshalMachineState", Ordered, func() {
		var (
			machineState *MachineState
			state        []byte
		)

		BeforeAll(func() {
			machineState = &MachineState{
				MachineDeployments: map[string]*MachineDeploymentState{
					"shoot--foo--bar-worker-z1": {
						Replicas: 3,
						MachineSets: []machinev1alpha1.MachineSet{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "shoot--foo--bar-worker-z1-hash1",
									Namespace: "shoot--foo--bar",
								},
								Spec: machinev1alpha1.MachineSetSpec{
									Replicas: 3,
									MachineClass: machinev1alpha1.ClassSpec{
										Name: "shoot--foo--bar-worker-z1-hash1",
									},
									Template: machinev1alpha1.MachineTemplateSpec{
										Spec: machinev1alpha1.MachineSpec{
											Class: machinev1alpha1.ClassSpec{
												Name: "shoot--foo--bar-worker-z1-hash1",
											},
										},
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "shoot--foo--bar-worker-z1-hash2",
									Namespace: "shoot--foo--bar",
								},
								Spec: machinev1alpha1.MachineSetSpec{
									Replicas: 3,
									MachineClass: machinev1alpha1.ClassSpec{
										Name: "shoot--foo--bar-worker-z1-hash2",
									},
									Template: machinev1alpha1.MachineTemplateSpec{
										Spec: machinev1alpha1.MachineSpec{
											Class: machinev1alpha1.ClassSpec{
												Name: "shoot--foo--bar-worker-z1-hash2",
											},
										},
									},
								},
							},
						},
						Machines: []machinev1alpha1.Machine{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "shoot--foo--bar-worker-z1-hash1-abcde",
									Namespace: "shoot--foo--bar",
								},
								Spec: machinev1alpha1.MachineSpec{
									Class: machinev1alpha1.ClassSpec{
										Name: "shoot--foo--bar-worker-z1-hash1-abcde",
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "shoot--foo--bar-worker-z1-hash2-abcde",
									Namespace: "shoot--foo--bar",
								},
								Spec: machinev1alpha1.MachineSpec{
									Class: machinev1alpha1.ClassSpec{
										Name: "shoot--foo--bar-worker-z1-hash2-abcde",
									},
								},
							},
						},
					},
				},
			}
		})

		It("should marshal the MachineState", func() {
			var err error
			state, err = MarshalMachineState(machineState)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should unmarshal the MachineState", func() {
			machineStateAfter, err := UnmarshalMachineState(state)
			Expect(err).NotTo(HaveOccurred())
			Expect(machineState).To(DeepEqual(machineStateAfter))
		})
	})
})
