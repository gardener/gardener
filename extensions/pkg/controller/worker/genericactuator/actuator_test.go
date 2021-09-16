// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	workerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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

	var (
		conditionRollingUpdateInProgress = gardencorev1beta1.Condition{
			Type:   extensionsv1alpha1.WorkerRollingUpdate,
			Status: gardencorev1beta1.ConditionTrue,
			Reason: ReasonRollingUpdateProgressing,
		}
		conditionNoRollingUpdate = gardencorev1beta1.Condition{
			Type:   extensionsv1alpha1.WorkerRollingUpdate,
			Status: gardencorev1beta1.ConditionTrue,
			Reason: ReasonRollingUpdateProgressing,
		}
	)

	DescribeTable("#buildRollingUpdateCondition", func(conditions []gardencorev1beta1.Condition, rollingUpdate bool, expectedConditionStatus gardencorev1beta1.ConditionStatus, expectedConditionReason string) {
		condition, err := buildRollingUpdateCondition([]gardencorev1beta1.Condition{}, rollingUpdate)
		Expect(err).ToNot(HaveOccurred())
		Expect(condition.Type).To(Equal(extensionsv1alpha1.WorkerRollingUpdate))
		Expect(condition.Status).To(Equal(expectedConditionStatus))
		Expect(condition.Reason).To(Equal(expectedConditionReason))
	},
		Entry("should update worker conditions with rolling update", []gardencorev1beta1.Condition{}, true, gardencorev1beta1.ConditionTrue, ReasonRollingUpdateProgressing),
		Entry("should update worker conditions with rolling update with pre-existing condition", []gardencorev1beta1.Condition{conditionNoRollingUpdate}, true, gardencorev1beta1.ConditionTrue, ReasonRollingUpdateProgressing),

		Entry("no rolling update", []gardencorev1beta1.Condition{}, false, gardencorev1beta1.ConditionFalse, ReasonNoRollingUpdate),
		Entry("should update worker conditions with rolling update with pre-existing condition", []gardencorev1beta1.Condition{conditionRollingUpdateInProgress}, false, gardencorev1beta1.ConditionFalse, ReasonNoRollingUpdate),
	)

	Describe("#isMachineControllerStuck", func() {
		var (
			machineDeploymentName           = "machine-deployment-1"
			machineDeploymentOwnerReference = []metav1.OwnerReference{{Name: machineDeploymentName, Kind: workerhelper.MachineDeploymentKind}}

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

	Describe("#updateCloudCredentialsForUnwantedMachineDeployments", func() {
		const (
			workerNamespace           = "test-ns"
			machineClassSecretName1   = "machine-class-secret1"
			machineClassSecretName2   = "machine-class-secret2"
			nonMachineClassSecretName = "non-machine-class-secret"
		)

		var (
			all    []runtime.Object
			logger = log.Log.WithName("test")

			usernameKey   = "username"
			usernameValue = []byte("username")

			cloudCredentials = map[string][]byte{
				usernameKey: usernameValue,
			}
			machineClassSecret1 = &corev1.Secret{
				Data: map[string][]byte{
					"key": []byte("value"),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineClassSecretName1,
					Namespace: workerNamespace,
					Labels:    map[string]string{"gardener.cloud/purpose": "machineclass"},
				},
			}
			machineClassSecret2 = &corev1.Secret{
				Data: map[string][]byte{
					usernameKey: []byte("outadated-username"),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineClassSecretName2,
					Namespace: workerNamespace,
					Labels:    map[string]string{"gardener.cloud/purpose": "machineclass"},
				},
			}
			nonMachineClassSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nonMachineClassSecretName,
					Namespace: workerNamespace,
				},
			}
		)

		BeforeEach(func() {
			all = []runtime.Object{
				machineClassSecret1,
				machineClassSecret2,
				nonMachineClassSecret,
			}
		})

		It("should update the cloud credentials for all machine class secret", func() {
			a := &genericActuator{client: fake.NewClientBuilder().WithRuntimeObjects(all...).Build()}
			secret1 := &corev1.Secret{}
			secret2 := &corev1.Secret{}
			secret3 := &corev1.Secret{}

			Expect(a.updateCloudCredentialsInAllMachineClassSecrets(context.TODO(), logger, cloudCredentials, workerNamespace)).ToNot(HaveOccurred())
			Expect(a.client.Get(context.TODO(), client.ObjectKey{Name: machineClassSecretName1, Namespace: workerNamespace}, secret1)).ToNot(HaveOccurred())
			Expect(a.client.Get(context.TODO(), client.ObjectKey{Name: machineClassSecretName2, Namespace: workerNamespace}, secret2)).ToNot(HaveOccurred())
			Expect(a.client.Get(context.TODO(), client.ObjectKey{Name: nonMachineClassSecretName, Namespace: workerNamespace}, secret3)).ToNot(HaveOccurred())

			for key, value := range cloudCredentials {
				v, ok := secret1.Data[key]
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(value))

				v, ok = secret2.Data[key]
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(value))

				_, ok = secret3.Data[key]
				Expect(ok).To(BeFalse())
			}
		})
	})

	Describe("#restoreMachineSetsAndMachines", func() {
		var (
			ctx    = context.TODO()
			logger = log.Log.WithName("test")

			c client.Client
			a *genericActuator

			machineDeployments worker.MachineDeployments
			expectedMachineSet machinev1alpha1.MachineSet
			expectedMachine    machinev1alpha1.Machine
		)

		BeforeEach(func() {
			s := runtime.NewScheme()
			Expect(machinev1alpha1.AddToScheme(s)).To(Succeed())
			c = fake.NewClientBuilder().WithScheme(s).Build()
			a = &genericActuator{client: c}

			expectedMachineSet = machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machineset",
					Namespace: "test-ns",
				},
			}

			expectedMachine = machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine",
					Namespace: "test-ns",
				},
				Status: machinev1alpha1.MachineStatus{
					Node: "node-name",
				},
			}

			machineDeployments = worker.MachineDeployments{
				{
					State: &worker.MachineDeploymentState{
						Replicas: 1,
						MachineSets: []machinev1alpha1.MachineSet{
							expectedMachineSet,
						},
						Machines: []machinev1alpha1.Machine{
							expectedMachine,
						},
					},
				},
			}
		})

		It("should deploy machinesets and machines present in the machine deployments' state", func() {
			Expect(a.restoreMachineSetsAndMachines(ctx, logger, machineDeployments)).To(Succeed())

			createdMachine := &machinev1alpha1.Machine{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(&expectedMachine), createdMachine)).To(Succeed())
			Expect(createdMachine.Status).To(Equal(expectedMachine.Status))

			createdMachineSet := &machinev1alpha1.MachineSet{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(&expectedMachineSet), createdMachineSet)).To(Succeed())
		})

		It("should update the machine status if machineset and machine already exist", func() {
			Expect(c.Create(ctx, (&expectedMachine).DeepCopy())).To(Succeed())
			Expect(c.Create(ctx, (&expectedMachineSet).DeepCopy())).To(Succeed())
			Expect(a.restoreMachineSetsAndMachines(ctx, logger, machineDeployments)).To(Succeed())

			createdMachine := &machinev1alpha1.Machine{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(&expectedMachine), createdMachine)).To(Succeed())
			Expect(expectedMachine.Status).To(Equal(expectedMachine.Status))

			createdMachineSet := &machinev1alpha1.MachineSet{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(&expectedMachineSet), createdMachineSet)).To(Succeed())
		})
	})
})
