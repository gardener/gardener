// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsworkerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("Actuator", func() {
	Describe("#listMachineClassSecrets", func() {
		const (
			ns = "test-ns"
		)

		var (
			expected *corev1.Secret
			all      []runtime.Object
		)

		BeforeEach(func() {
			expected = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machineclass-secret1",
					Namespace: ns,
					Labels:    map[string]string{"gardener.cloud/purpose": "machineclass"},
				},
			}
			all = []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machineclass-secret3",
						Namespace: "other-ns",
						Labels:    map[string]string{"gardener.cloud/purpose": "machineclass"},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machineclass-secret4",
						Namespace: ns,
					},
				},
				expected,
			}
		})

		It("should return secrets matching the label selector", func() {
			a := &genericActuator{client: fake.NewClientBuilder().WithRuntimeObjects(all...).Build()}
			actual, err := a.listMachineClassSecrets(context.TODO(), ns)

			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Items).To(HaveLen(1))
			Expect(actual.Items[0].Name).To(Equal(expected.Name))
		})
	})

	Describe("#removeWantedDeploymentWithoutState", func() {
		var (
			mdWithoutState            = worker.MachineDeployment{ClassName: "gcp", Name: "md-without-state"}
			mdWithStateAndMachineSets = worker.MachineDeployment{ClassName: "gcp", Name: "md-with-state-machinesets", State: &worker.MachineDeploymentState{Replicas: 1, MachineSets: []machinev1alpha1.MachineSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machineSet",
					},
				},
			}}}
			mdWithEmptyState = worker.MachineDeployment{ClassName: "gcp", Name: "md-with-state", State: &worker.MachineDeploymentState{Replicas: 1, MachineSets: []machinev1alpha1.MachineSet{}}}
		)

		It("should not panic for MachineDeployments without state", func() {
			removeWantedDeploymentWithoutState(worker.MachineDeployments{mdWithoutState})
		})

		It("should not panic for empty slice of MachineDeployments", func() {
			removeWantedDeploymentWithoutState(make(worker.MachineDeployments, 0))
		})

		It("should not panic MachineDeployments is nil", func() {
			removeWantedDeploymentWithoutState(nil)
		})

		It("should not return nil if MachineDeployments are reduced to zero", func() {
			Expect(removeWantedDeploymentWithoutState(worker.MachineDeployments{mdWithoutState})).NotTo(BeNil())
		})

		It("should return only MachineDeployments with states", func() {
			reducedMDs := removeWantedDeploymentWithoutState(worker.MachineDeployments{mdWithoutState, mdWithStateAndMachineSets})

			Expect(len(reducedMDs)).To(Equal(1))
			Expect(reducedMDs[0]).To(Equal(mdWithStateAndMachineSets))
		})

		It("should reduce the lenght to one", func() {
			reducedMDs := removeWantedDeploymentWithoutState(worker.MachineDeployments{mdWithoutState, mdWithStateAndMachineSets, mdWithEmptyState})

			Expect(len(reducedMDs)).To(Equal(1))
			Expect(reducedMDs[0]).To(Equal(mdWithStateAndMachineSets))
		})
	})

	Describe("#isMachineControllerStuck", func() {
		var (
			machineDeploymentName           = "machine-deployment-1"
			machineDeploymentOwnerReference = []metav1.OwnerReference{{Name: machineDeploymentName, Kind: extensionsworkerhelper.MachineDeploymentKind}}

			machineClassName      = "machine-class-new"
			machineDeploymentSpec = machinev1alpha1.MachineDeploymentSpec{
				Template: machinev1alpha1.MachineTemplateSpec{
					Spec: machinev1alpha1.MachineSpec{
						Class: machinev1alpha1.ClassSpec{
							Name: machineClassName,
						},
					},
				},
			}

			machineDeployment = machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:       machineDeploymentName,
					Finalizers: []string{"machine.sapcloud.io/machine-controller-manager"},
				},
				Spec: machineDeploymentSpec,
			}

			machineDeploymentTooYoung = machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:              machineDeploymentName,
					Finalizers:        []string{"machine.sapcloud.io/machine-controller-manager"},
					CreationTimestamp: metav1.Now(),
				},
				Spec: machineDeploymentSpec,
			}

			machineDeploymentNoFinalizer = machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other",
				},
				Spec: machineDeploymentSpec,
			}
			machineDeployments = []machinev1alpha1.MachineDeployment{
				machineDeployment,
			}

			machineSetSpec = machinev1alpha1.MachineSetSpec{
				Template: machinev1alpha1.MachineTemplateSpec{
					Spec: machinev1alpha1.MachineSpec{
						Class: machinev1alpha1.ClassSpec{
							Name: machineClassName,
						},
					},
				},
			}

			matchingMachineSet = machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: machineDeploymentOwnerReference,
					Name:            "matching-machine-set",
				},
				Spec: machineSetSpec,
			}

			machineSetOtherMachineClass = machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: machineDeploymentOwnerReference,
					Name:            "machine-set-old",
				},
				Spec: machinev1alpha1.MachineSetSpec{
					Template: machinev1alpha1.MachineTemplateSpec{
						Spec: machinev1alpha1.MachineSpec{
							Class: machinev1alpha1.ClassSpec{
								Name: "machine-class-old",
							},
						},
					},
				},
			}

			machineSetOtherOwner = machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{Name: "machine-deployment-2"}},
					Name:            "other-machine-set",
				},
			}
		)

		DescribeTable("#isMachineControllerStuck", func(machineSets []machinev1alpha1.MachineSet, machineDeployments []machinev1alpha1.MachineDeployment, expectedIsStuck bool) {
			stuck, _ := isMachineControllerStuck(machineSets, machineDeployments)
			Expect(stuck).To(Equal(expectedIsStuck))
		},

			Entry("should not be stuck", []machinev1alpha1.MachineSet{matchingMachineSet}, machineDeployments, false),
			Entry("should not be stuck - machine deployment does not have mcm finalizer", []machinev1alpha1.MachineSet{matchingMachineSet}, []machinev1alpha1.MachineDeployment{machineDeploymentNoFinalizer, machineDeployment}, false),
			Entry("should not be stuck - machine deployment is too young to be considered for the check", []machinev1alpha1.MachineSet{}, []machinev1alpha1.MachineDeployment{machineDeploymentTooYoung}, false),
			Entry("should be stuck - machine set does not have matching matching class", []machinev1alpha1.MachineSet{machineSetOtherMachineClass}, machineDeployments, true),
			Entry("should be stuck - no machine set with matching owner reference", []machinev1alpha1.MachineSet{machineSetOtherOwner}, machineDeployments, true),
		)
	})

	Describe("#restoreMachineSetsAndMachines", func() {
		var (
			ctx    = context.TODO()
			logger = log.Log.WithName("test")

			mockCtrl   *gomock.Controller
			mockClient *mockclient.MockClient

			a *genericActuator

			machineDeployments worker.MachineDeployments
			expectedMachineSet machinev1alpha1.MachineSet
			expectedMachine1   machinev1alpha1.Machine
			expectedMachine2   machinev1alpha1.Machine

			alreadyExistsError = apierrors.NewAlreadyExists(schema.GroupResource{Resource: "Machines"}, "machine")
		)

		BeforeEach(func() {
			mockCtrl = gomock.NewController(GinkgoT())
			mockClient = mockclient.NewMockClient(mockCtrl)

			a = &genericActuator{client: mockClient}

			expectedMachineSet = machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machineset",
					Namespace: "test-ns",
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

			machineDeployments = worker.MachineDeployments{
				{
					State: &worker.MachineDeploymentState{
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

		AfterEach(func() {
			mockCtrl.Finish()
		})

		It("should deploy machinesets and machines present in the machine deployments' state", func() {
			mockClient.EXPECT().Create(ctx, &expectedMachineSet)
			mockClient.EXPECT().Create(ctx, &expectedMachine1)
			mockClient.EXPECT().Create(ctx, &expectedMachine2)

			Expect(restoreMachineSetsAndMachines(ctx, logger, a.client, machineDeployments)).To(Succeed())
		})

		It("should not return error if machineset and machines already exist", func() {
			mockClient.EXPECT().Create(ctx, &expectedMachineSet).Return(alreadyExistsError)
			mockClient.EXPECT().Create(ctx, &expectedMachine1).Return(alreadyExistsError)
			mockClient.EXPECT().Create(ctx, &expectedMachine2).Return(alreadyExistsError)

			Expect(restoreMachineSetsAndMachines(ctx, logger, a.client, machineDeployments)).To(Succeed())
		})
	})
})
