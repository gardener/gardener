// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
			worker = &extensionsv1alpha1.Worker{}

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

		Context("read machine-state from Worker status", func() {
			BeforeEach(func() {
				fakeGardenClient = nil // ensure that GardenClient is not used
				worker.Status.State = &runtime.RawExtension{Raw: machineStateCompressed}
			})

			It("should do nothing because machine state data in Worker is null", func() {
				worker.Status.State = &runtime.RawExtension{Raw: []byte(`{"state": null}`)}

				Expect(addStateToMachineDeployment(ctx, log, fakeGardenClient, shoot, worker, wantedMachineDeployments)).To(Succeed())

				Expect(wantedMachineDeployments[0].State).To(BeNil())
				Expect(wantedMachineDeployments[1].State).To(BeNil())
				Expect(wantedMachineDeployments[2].State).To(BeNil())
			})

			It("should read the machine state from the Worker", func() {
				Expect(addStateToMachineDeployment(ctx, log, fakeGardenClient, shoot, worker, wantedMachineDeployments)).To(Succeed())

				Expect(wantedMachineDeployments[0].State).To(Equal(stateDeployment1))
				Expect(wantedMachineDeployments[1].State).To(Equal(stateDeployment2))
				Expect(wantedMachineDeployments[2].State).To(BeNil())
			})
		})

		Context("fall back to ShootState", func() {
			It("should do nothing because machine state does not exist in ShootState", func() {
				Expect(addStateToMachineDeployment(ctx, log, fakeGardenClient, shoot, worker, wantedMachineDeployments)).To(Succeed())

				Expect(wantedMachineDeployments[0].State).To(BeNil())
				Expect(wantedMachineDeployments[1].State).To(BeNil())
				Expect(wantedMachineDeployments[2].State).To(BeNil())
			})

			It("should do nothing because machine state data in ShootState is null", func() {
				patch := client.MergeFrom(shootState.DeepCopy())
				shootState.Spec = gardencorev1beta1.ShootStateSpec{
					Gardener: []gardencorev1beta1.GardenerResourceData{{
						Name: "machine-state",
						Type: "machine-state",
						Data: runtime.RawExtension{Raw: []byte(`{"state": null}`)},
					}},
				}
				Expect(fakeGardenClient.Patch(ctx, shootState, patch)).To(Succeed())

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
		})
	})

	Describe("#restoreMachineSetsAndMachines", func() {
		var (
			ctx            = context.Background()
			log            = logr.Discard()
			fakeSeedClient client.Client

			machineDeployments extensionsworkercontroller.MachineDeployments
			expectedMachineSet machinev1alpha1.MachineSet
			expectedMachine1   machinev1alpha1.Machine
			expectedMachine2   machinev1alpha1.Machine
		)

		BeforeEach(func() {
			fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			expectedMachineSet = machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machineset",
					Namespace: "test-ns",
					Labels: map[string]string{
						"name": "pool1",
					},
				},
			}

			expectedMachine1 = machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine",
					Namespace: "test-ns",
					Labels: map[string]string{
						"node": "node-name-one",
					},
				},
			}

			expectedMachine2 = machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-two",
					Namespace: "test-ns",
					Labels: map[string]string{
						"node": "node-name-two",
					},
				},
			}

			machineDeployments = extensionsworkercontroller.MachineDeployments{
				{
					State: &shootstate.MachineDeploymentState{
						Replicas: 2,
						MachineSets: []machinev1alpha1.MachineSet{
							expectedMachineSet,
						},
						Machines: []machinev1alpha1.Machine{
							expectedMachine1,
							expectedMachine2,
						},
					},
				},
			}
		})

		test := func() {
			Expect(restoreMachineSetsAndMachines(ctx, log, fakeSeedClient, machineDeployments)).To(Succeed())

			var actualMachineSet machinev1alpha1.MachineSet
			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(&expectedMachineSet), &actualMachineSet)).To(Succeed())
			expectedMachineSet.ResourceVersion = "1"
			Expect(actualMachineSet).To(Equal(expectedMachineSet))

			for _, expectedMachine := range []machinev1alpha1.Machine{expectedMachine1, expectedMachine2} {
				var actualMachine machinev1alpha1.Machine
				Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(&expectedMachine), &actualMachine)).To(Succeed(), expectedMachine.Name+" should be retrieved successfully")
				expectedMachine.ResourceVersion = "1"
				Expect(actualMachine).To(Equal(expectedMachine), "actual should be equal to expected Machine"+expectedMachine.Name)
			}
		}

		When("MachineSets and Machines do not exist", func() {
			It("should deploy MachineSets and Machines present in the MachineDeployments' state", func() {
				test()
			})
		})

		When("MachineSet and Machines already exist", func() {
			BeforeEach(func() {
				expectedMachineSet.Labels = map[string]string{"foo": "bar"}
				Expect(fakeSeedClient.Create(ctx, &expectedMachineSet)).To(Succeed())

				expectedMachine1.Labels = map[string]string{"foo": "bar"}
				expectedMachine2.Labels = map[string]string{"foo": "bar"}
				for _, expectedMachine := range []machinev1alpha1.Machine{expectedMachine1, expectedMachine2} {
					Expect(fakeSeedClient.Create(ctx, &expectedMachine)).To(Succeed(), expectedMachine.Name+" should be created successfully")
				}
			})

			It("should not re-deploy MachineSets and Machines present in the MachineDeployments' state", func() {
				test()
			})
		})
	})
})
