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
	"errors"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	workerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			a := &genericActuator{client: fake.NewFakeClient(all...)}
			actual, err := a.listMachineClassSecrets(context.TODO(), ns)

			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Items).To(HaveLen(1))
			Expect(actual.Items[0].Name).To(Equal(expected.Name))
		})
	})

	Describe("#CleanupLeakedClusterRoles", func() {
		var (
			ctrl *gomock.Controller

			ctx = context.TODO()
			c   *mockclient.MockClient

			providerName = "provider-foo"
			fakeErr      = errors.New("fake")

			namespace1              = "abcd"
			namespace2              = "efgh"
			namespace3              = "ijkl"
			nonMatchingClusterRoles = []rbacv1.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "doesnotmatch"}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:provider-bar:%s:machine-controller-manager", namespace1)}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s", providerName, namespace1)}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:bar", providerName, namespace1)}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:machine-controller-manager", providerName)}},
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return an error while listing the clusterroles", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).Return(fakeErr)

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Equal(fakeErr))
		})

		It("should return an error while listing the namespaces", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}))
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).Return(fakeErr)

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Equal(fakeErr))
		})

		It("should do nothing because clusterrole list is empty", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}))
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{}))

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should do nothing because clusterrole list doesn't contain matches", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{Items: nonMatchingClusterRoles}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{}))

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should do nothing because no orphaned clusterroles found", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{
					Items: append(nonMatchingClusterRoles, rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace1)},
					}),
				}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).DoAndReturn(func(_ context.Context, list *corev1.NamespaceList, _ ...client.ListOption) error {
				*list = corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: namespace1}},
					},
				}
				return nil
			})

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should delete the orphaned clusterroles", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{
					Items: append(
						nonMatchingClusterRoles,
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace1)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}},
					),
				}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).DoAndReturn(func(_ context.Context, list *corev1.NamespaceList, _ ...client.ListOption) error {
				*list = corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: namespace1}},
					},
				}
				return nil
			})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}})

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should return the error occurred during orphaned clusterrole deletion", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{
					Items: append(
						nonMatchingClusterRoles,
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace1)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}},
					),
				}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).DoAndReturn(func(_ context.Context, list *corev1.NamespaceList, _ ...client.ListOption) error {
				*list = corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: namespace1}},
					},
				}
				return nil
			})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}}).Return(fakeErr)

			err := CleanupLeakedClusterRoles(ctx, c, providerName)

			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(Equal([]error{fakeErr}))
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
})
