// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package care

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Health", func() {
	Describe("#checkNodesScalingUp", func() {
		It("should return true if number of ready nodes equal number of desired machines", func() {
			Expect(checkNodesScalingUp(nil, 1, 1)).To(Succeed())
		})

		It("should return an error if not enough machine objects as desired were created", func() {
			Expect(checkNodesScalingUp(&machinev1alpha1.MachineList{}, 0, 1)).To(MatchError(ContainSubstring("not enough machine objects created yet")))
		})

		It("should return an error when detecting erroneous machines", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineUnknown},
						},
					},
				},
			}

			Expect(checkNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is erroneous")))
		})

		It("should return an error when not enough ready nodes are registered", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineRunning},
						},
					},
				},
			}

			Expect(checkNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("not enough ready worker nodes registered in the cluster")))
		})

		It("should return progressing when detecting a regular scale up (pending status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachinePending},
						},
					},
				},
			}

			Expect(checkNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is provisioning and should join the cluster soon")))
		})

		It("should return progressing when detecting a regular scale up (no status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{},
				},
			}

			Expect(checkNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is provisioning and should join the cluster soon")))
		})
	})

	Describe("#checkNodesScalingDown", func() {
		It("should return true if number of registered nodes equal number of desired machines", func() {
			Expect(checkNodesScalingDown(nil, nil, 1, 1)).To(Succeed())
		})

		It("should return an error if the machine for a cordoned node is not found", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{Unschedulable: true}},
				},
			}

			Expect(checkNodesScalingDown(&machinev1alpha1.MachineList{}, nodeList, 2, 1)).To(MatchError(ContainSubstring("machine object for cordoned node \"\" not found")))
		})

		It("should return an error if the machine for a cordoned node is not deleted", func() {
			var (
				nodeName = "foo"

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			Expect(checkNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("found but corresponding machine object does not have a deletion timestamp")))
		})

		It("should return an error if there are more nodes then machines", func() {
			Expect(checkNodesScalingDown(&machinev1alpha1.MachineList{}, &corev1.NodeList{Items: []corev1.Node{{}}}, 2, 1)).To(MatchError(ContainSubstring("too many worker nodes are registered. Exceeding maximum desired machine count")))
		})

		It("should return progressing for a regular scale down", func() {
			var (
				nodeName          = "foo"
				deletionTimestamp = metav1.Now()

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp, Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			Expect(checkNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("is waiting to be completely drained from pods")))
		})

		It("should ignore node not managed by MCM and return progressing for a regular scale down", func() {
			var (
				nodeName          = "foo"
				deletionTimestamp = metav1.Now()

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp, Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "bar",
								Annotations: map[string]string{"node.machine.sapcloud.io/not-managed-by-mcm": "1"},
							},
						},
					},
				}
			)

			Expect(checkNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("is waiting to be completely drained from pods")))
		})
	})
})
