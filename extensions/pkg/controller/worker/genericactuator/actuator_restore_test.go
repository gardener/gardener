// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwr.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

var _ = Describe("ActuatorRestore", func() {
	Describe("#addStateToMachineDeployment", func() {
		var (
			ctx              = context.Background()
			log              = logr.Discard()
			fakeGardenClient client.Client

			shoot                    *gardencorev1beta1.Shoot
			shootState               *gardencorev1beta1.ShootState
			worker                   *extensionsv1alpha1.Worker
			wantedMachineDeployments extensionsworkercontroller.MachineDeployments

			stateDeployment1 = &shootstate.MachineDeploymentState{
				Replicas:    1,
				MachineSets: []machinev1alpha1.MachineSet{{ObjectMeta: metav1.ObjectMeta{Name: "deploy1-set1"}}},
				Machines:    []machinev1alpha1.Machine{{ObjectMeta: metav1.ObjectMeta{Name: "deploy1-machine1"}}},
			}
			stateDeployment2 = &shootstate.MachineDeploymentState{
				Replicas:    2,
				MachineSets: []machinev1alpha1.MachineSet{{ObjectMeta: metav1.ObjectMeta{Name: "deploy2-set1"}}},
				Machines:    []machinev1alpha1.Machine{{ObjectMeta: metav1.ObjectMeta{Name: "deploy2-machine1"}}},
			}
			machineState = &shootstate.MachineState{MachineDeployments: map[string]*shootstate.MachineDeploymentState{
				"deploy1": stateDeployment1,
				"deploy2": stateDeployment2,
			}}
			machineStateRaw        []byte
			machineStateCompressed []byte
		)

		BeforeEach(func() {
			fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

			shoot = &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: "foo"}}
			shootState = &gardencorev1beta1.ShootState{ObjectMeta: metav1.ObjectMeta{Name: shoot.Name, Namespace: shoot.Namespace}}
			worker = &extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--foo--bar"}}
			wantedMachineDeployments = []extensionsworkercontroller.MachineDeployment{
				{Name: "deploy1"},
				{Name: "deploy2"},
				{Name: "deploy3"},
			}

			var err error
			machineStateRaw, err = json.Marshal(machineState)
			Expect(err).NotTo(HaveOccurred())

			var buffer bytes.Buffer
			gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
			Expect(err).NotTo(HaveOccurred())
			_, err = gzipWriter.Write(machineStateRaw)
			Expect(err).NotTo(HaveOccurred())
			Expect(gzipWriter.Close()).To(Succeed())
			machineStateCompressed = []byte(`{"state":"` + utils.EncodeBase64(buffer.Bytes()) + `"}`)

			Expect(fakeGardenClient.Create(ctx, shootState)).To(Succeed())
		})

		It("should do nothing because neither machine state exists in ShootState nor .status.state field is set", func() {
			Expect(addStateToMachineDeployment(ctx, log, fakeGardenClient, shoot, worker, wantedMachineDeployments)).To(Succeed())

			Expect(wantedMachineDeployments[0].State).To(BeNil())
			Expect(wantedMachineDeployments[1].State).To(BeNil())
			Expect(wantedMachineDeployments[2].State).To(BeNil())
		})

		It("should fetch the machine state from the ShootState", func() {
			patch := client.MergeFrom(shootState.DeepCopy())
			shootState.Spec = gardencorev1beta1.ShootStateSpec{
				Gardener: []gardencorev1beta1.GardenerResourceData{{
					Name: "machine-state",
					Type: "machine-state",
					Data: runtime.RawExtension{Raw: machineStateCompressed},
				}},
			}
			Expect(fakeGardenClient.Patch(ctx, shootState, patch)).To(Succeed())

			Expect(addStateToMachineDeployment(ctx, log, fakeGardenClient, shoot, worker, wantedMachineDeployments)).To(Succeed())

			Expect(wantedMachineDeployments[0].State).To(Equal(stateDeployment1))
			Expect(wantedMachineDeployments[1].State).To(Equal(stateDeployment2))
			Expect(wantedMachineDeployments[2].State).To(BeNil())
		})

		It("should fetch the state from the Worker's .status.state field when machine state does not exist in ShootState", func() {
			worker.Status.State = &runtime.RawExtension{Raw: machineStateRaw}

			Expect(addStateToMachineDeployment(ctx, log, fakeGardenClient, shoot, worker, wantedMachineDeployments)).To(Succeed())

			Expect(wantedMachineDeployments[0].State).To(Equal(stateDeployment1))
			Expect(wantedMachineDeployments[1].State).To(Equal(stateDeployment2))
			Expect(wantedMachineDeployments[2].State).To(BeNil())
		})
	})
})
