// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/destroy"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

var _ = Describe("Shoot", func() {
	Describe("#RemoveNodeLabelsFromMachineState", func() {
		It("should remove the node label from the machines", func() {
			raw, err := shootstate.MarshalMachineState(&shootstate.MachineState{
				MachineDeployments: map[string]*shootstate.MachineDeploymentState{
					"worker": {
						Replicas: 1,
						Machines: []machinev1alpha1.Machine{{ObjectMeta: metav1.ObjectMeta{
							Name:   "machine",
							Labels: map[string]string{"node": "machine", "name": "worker"},
						}}},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			shootState := &gardencorev1beta1.ShootState{Spec: gardencorev1beta1.ShootStateSpec{Gardener: []gardencorev1beta1.GardenerResourceData{
				{Name: "secret", Type: "secret", Data: runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}},
				{Name: "machine-state", Type: "machine-state", Data: runtime.RawExtension{Raw: raw}},
			}}}

			Expect(RemoveNodeLabelsFromMachineState(shootState)).To(Succeed())

			Expect(shootState.Spec.Gardener[0].Data.Raw).To(Equal([]byte(`{"foo":"bar"}`)))
			machineState, err := shootstate.UnmarshalMachineState(shootState.Spec.Gardener[1].Data.Raw)
			Expect(err).NotTo(HaveOccurred())
			Expect(machineState.MachineDeployments["worker"].Machines).To(ConsistOf(
				HaveField("ObjectMeta.Labels", Equal(map[string]string{"name": "worker"})),
			))
		})

		It("should succeed without machine state", func() {
			Expect(RemoveNodeLabelsFromMachineState(&gardencorev1beta1.ShootState{})).To(Succeed())
		})
	})
})
