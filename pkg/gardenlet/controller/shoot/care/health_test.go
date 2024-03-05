// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("health check", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		fakeClock  = testclock.NewFakeClock(time.Now())

		condition gardencorev1beta1.Condition

		seedNamespace     = "shoot--foo--bar"
		kubernetesVersion = semver.MustParse("1.27.3")

		shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{
							Name: "worker",
						},
					},
				},
			},
		}

		workerlessShoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{},
				},
			},
		}
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClock = testclock.NewFakeClock(time.Now())
		condition = gardencorev1beta1.Condition{
			Type:               "test",
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
	})

	Describe("#ComputeRequiredControlPlaneDeployments", func() {
		var (
			workerlessDepoymentNames = []interface{}{
				"gardener-resource-manager",
				"kube-apiserver",
				"kube-controller-manager",
			}
			commonDeploymentNames = append(workerlessDepoymentNames, "kube-scheduler", "machine-controller-manager")
		)

		tests := func(shoot *gardencorev1beta1.Shoot, names []interface{}, isWorkerless bool) {
			It("should return expected deployments for shoot", func() {
				deploymentNames, err := ComputeRequiredControlPlaneDeployments(shoot)

				Expect(err).ToNot(HaveOccurred())
				Expect(deploymentNames.UnsortedList()).To(ConsistOf(names...))
			})

			It("should return expected deployments for shoot with Cluster Autoscaler", func() {
				if isWorkerless {
					return
				}

				expectedDeploymentNames := append(names, "cluster-autoscaler")
				shootWithCA := shoot.DeepCopy()
				shootWithCA.Spec.Provider.Workers = []gardencorev1beta1.Worker{
					{
						Name:    "worker",
						Minimum: 0,
						Maximum: 1,
					},
				}

				deploymentNames, err := ComputeRequiredControlPlaneDeployments(shootWithCA)

				Expect(err).ToNot(HaveOccurred())
				Expect(deploymentNames.UnsortedList()).To(ConsistOf(expectedDeploymentNames...))
			})

			It("should return expected deployments for shoot with VPA", func() {
				expectedDeploymentNames := names
				if !isWorkerless {
					expectedDeploymentNames = append(expectedDeploymentNames, "vpa-admission-controller", "vpa-recommender", "vpa-updater")
				}

				shootWithVPA := shoot.DeepCopy()
				shootWithVPA.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					VerticalPodAutoscaler: &gardencorev1beta1.VerticalPodAutoscaler{
						Enabled: true,
					},
				}

				deploymentNames, err := ComputeRequiredControlPlaneDeployments(shootWithVPA)

				Expect(err).ToNot(HaveOccurred())
				Expect(deploymentNames.UnsortedList()).To(ConsistOf(expectedDeploymentNames...))
			})

		}

		Context("shoot", func() {
			tests(shoot, commonDeploymentNames, false)
		})

		Context("workerless shoot", func() {
			tests(workerlessShoot, workerlessDepoymentNames, true)
		})
	})

	Describe("#ComputeRequiredMonitoringStatefulSets", func() {
		It("should return expected statefulsets", func() {
			Expect(ComputeRequiredMonitoringStatefulSets().UnsortedList()).To(HaveExactElements("prometheus"))
		})
	})

	Describe("#ComputeRequiredMonitoringSeedDeployments", func() {
		It("should return expected deployments", func() {
			Expect(ComputeRequiredMonitoringSeedDeployments(shoot).UnsortedList()).To(HaveExactElements("kube-state-metrics"))
		})

		It("should return expected deployments for workerless shoot", func() {
			Expect(ComputeRequiredMonitoringSeedDeployments(workerlessShoot).UnsortedList()).To(BeEmpty())
		})
	})

	DescribeTable("#PardonCondition",
		func(condition gardencorev1beta1.Condition, lastOp *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, expected types.GomegaMatcher) {
			conditions := []gardencorev1beta1.Condition{condition}
			updatedConditions := PardonConditions(fakeClock, conditions, lastOp, lastErrors)
			Expect(updatedConditions).To(expected)
		},
		Entry("should pardon false ConditionStatus when the last operation is nil",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			nil,
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should pardon false ConditionStatus when the last operation is create processing",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should pardon false ConditionStatus when the last operation is delete processing",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeDelete,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should pardon false ConditionStatus when the last operation is processing and no last errors",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeReconcile,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should not pardon false ConditionStatus when the last operation is processing and last errors",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeReconcile,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			[]gardencorev1beta1.LastError{
				{Description: "error"},
			},
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("should not pardon false ConditionStatus when the last operation is create succeeded",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateSucceeded,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
	)

	Describe("#CheckClusterNodes", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			workerPoolName1            = "cpu-worker-1"
			workerPoolName2            = "cpu-worker-2"
			cloudConfigSecretChecksum1 = "foo"
			cloudConfigSecretChecksum2 = "foo"
			nodeName                   = "node1"
			oscSecretMeta              = map[string]metav1.ObjectMeta{
				workerPoolName1: {
					Name:        operatingsystemconfig.Key(workerPoolName1, kubernetesVersion, nil),
					Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
				},
				workerPoolName2: {
					Name:        "bar",
					Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName2},
					Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum2},
				},
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		DescribeTable("#CheckClusterNodes",
			func(k8sversion *semver.Version, nodes []corev1.Node, workerPools []gardencorev1beta1.Worker, oscSecretsExist bool, oscSecretMeta map[string]metav1.ObjectMeta, conditionMatcher types.GomegaMatcher) {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
					*list = corev1.NodeList{Items: nodes}
					return nil
				})

				Expect(fakeClient.Create(ctx, &machinev1alpha1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: seedNamespace},
					Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(len(nodes))},
				})).To(Succeed())

				roleValue := "cloud-config"
				c.EXPECT().Scheme().Return(kubernetes.ShootScheme)
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.InNamespace("kube-system"), client.MatchingLabels{"gardener.cloud/role": "operating-system-config"}, client.Limit(1)).DoAndReturn(func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					*list = metav1.PartialObjectMetadataList{}
					if oscSecretsExist {
						list.Items = append(list.Items, metav1.PartialObjectMetadata{})
					}
					return nil
				})

				if oscSecretsExist {
					roleValue = "operating-system-config"
				}

				secretListOptions := []client.ListOption{
					client.InNamespace(metav1.NamespaceSystem),
					client.MatchingLabels{"gardener.cloud/role": roleValue},
				}

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), secretListOptions).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{}
					for workerPoolName, m := range oscSecretMeta {
						meta := m.DeepCopy()
						// regenerate OSC secret key because it might be different when UseGardenerNodeAgent feature gate is enabled
						if strings.HasPrefix(meta.Name, "gardener-node-agent") || strings.HasPrefix(meta.Name, "cloud-config") {
							meta.Name = operatingsystemconfig.Key(workerPoolName, kubernetesVersion, nil)
						}

						list.Items = append(list.Items, corev1.Secret{
							ObjectMeta: *meta,
						})
					}
					return nil
				})

				shootObj := &shootpkg.Shoot{
					SeedNamespace:     seedNamespace,
					KubernetesVersion: k8sversion,
				}
				shootObj.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: workerPools,
						},
					},
				})

				health := NewHealth(
					logr.Discard(),
					shootObj,
					kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build(),
					nil,
					nil,
					fakeClock,
					nil,
					nil,
				)

				exitCondition, err := health.CheckClusterNodes(ctx, kubernetesfake.NewClientSetBuilder().WithClient(c).Build(), condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			// gardener-node-agent secret checks
			Entry("missing OSC secret checksum for a worker pool",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				true,
				nil,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("missing operating system config secret metadata for worker pool %q", workerPoolName1)))),
			Entry("missing OSC secret checksum for a worker pool when shoot has not been reconciled yet",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				false,
				nil,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("missing operating system config secret metadata for worker pool %q", workerPoolName1)))),
			Entry("no OSC node checksum for a worker pool",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				true,
				oscSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q hasn't been reported yet", nodeName)))),
			Entry("no OSC node checksum for a worker pool when shoot has not been reconciled yet",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				false,
				oscSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q hasn't been reported yet", nodeName)))),
			Entry("outdated OSC secret checksum for a worker pool",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, map[string]string{nodeagentv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: "outdated"}, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				true,
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.Key(workerPoolName1, kubernetesVersion, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q is outdated", nodeName)))),
			Entry("outdated OSC secret checksum for a worker pool when shoot has not been reconciled yet",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, map[string]string{nodeagentv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: "outdated"}, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				false,
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.Key(workerPoolName1, kubernetesVersion, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q is outdated", nodeName)))),
		)
	})

	Describe("#CheckingNodeAgentLease", func() {
		var (
			validLease = coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-node-agent-node1",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To(int32(40)),
				},
			}

			invalidLease = coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-node-agent-node1",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To(int32(-40)),
				},
			}

			nonrelatedLease = coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-node-agent-node2",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To(int32(40)),
				},
			}

			nodeList = corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
						},
					},
				},
			}
		)

		DescribeTable("#CheckingNodeAgentLease", func(lease coordinationv1.Lease, expected types.GomegaMatcher) {
			leaseList := coordinationv1.LeaseList{
				Items: []coordinationv1.Lease{
					lease,
				},
			}

			Expect(CheckNodeAgentLeases(&nodeList, &leaseList, fakeClock)).To(expected)
		},
			Entry("should return nil if there is a matching lease for node", validLease, BeNil()),
			// TODO(rfranzke): Remove this test-entry as soon as the UseGardenerNodeAgent feature gate gets removed.
			Entry("should return nil if no leases are present", nil, BeNil()),
			Entry("should return Error that node agent is not running if no matching lease could be found for node", nonrelatedLease, MatchError(ContainSubstring("not running"))),
			Entry("should return Error that node agent stopped running if the lease for the node is not valid anymore", invalidLease, MatchError(ContainSubstring("stopped running"))),
		)
	})

	Describe("#CheckNodesScalingUp", func() {
		It("should return true if number of ready nodes equal number of desired machines", func() {
			Expect(CheckNodesScalingUp(nil, 1, 1)).To(Succeed())
		})

		It("should return an error if not enough machine objects as desired were created", func() {
			Expect(CheckNodesScalingUp(&machinev1alpha1.MachineList{}, 0, 1)).To(MatchError(ContainSubstring("not enough machine objects created yet")))
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

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is erroneous")))
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

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("not enough ready worker nodes registered in the cluster")))
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

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is provisioning and should join the cluster soon")))
		})

		It("should return progressing when detecting a regular scale up (no status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{},
				},
			}

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is provisioning and should join the cluster soon")))
		})
	})

	Describe("#CheckNodesScalingDown", func() {
		It("should return true if number of registered nodes equal number of desired machines", func() {
			Expect(CheckNodesScalingDown(nil, nil, 1, 1)).To(Succeed())
		})

		It("should return an error if the machine for a cordoned node is not found", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{Unschedulable: true}},
				},
			}

			Expect(CheckNodesScalingDown(&machinev1alpha1.MachineList{}, nodeList, 2, 1)).To(MatchError(ContainSubstring("machine object for cordoned node \"\" not found")))
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

			Expect(CheckNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("found but corresponding machine object does not have a deletion timestamp")))
		})

		It("should return an error if there are more nodes then machines", func() {
			Expect(CheckNodesScalingDown(&machinev1alpha1.MachineList{}, &corev1.NodeList{Items: []corev1.Node{{}}}, 2, 1)).To(MatchError(ContainSubstring("too many worker nodes are registered. Exceeding maximum desired machine count")))
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

			Expect(CheckNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("is waiting to be completely drained from pods")))
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

			Expect(CheckNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("is waiting to be completely drained from pods")))
		})
	})

	Describe("ShootConditions", func() {
		Describe("#NewShootConditions", func() {
			It("should initialize all conditions", func() {
				conditions := NewShootConditions(fakeClock, &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{Name: "worker"}},
						},
					},
				})

				Expect(conditions.ConvertToSlice()).To(ConsistOf(
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})

			It("should initialize all conditions for workerless shoot", func() {
				conditions := NewShootConditions(fakeClock, &gardencorev1beta1.Shoot{})

				Expect(conditions.ConvertToSlice()).To(ConsistOf(
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})

			It("should only initialize missing conditions", func() {
				conditions := NewShootConditions(fakeClock, &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: "APIServerAvailable"},
							{Type: "Foo"},
						},
					},
				})

				Expect(conditions.ConvertToSlice()).To(ConsistOf(
					OfType("APIServerAvailable"),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})
		})

		Describe("#ConvertToSlice", func() {
			It("should return the expected conditions", func() {
				conditions := NewShootConditions(fakeClock, &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{Name: "worker"}},
						},
					},
				})

				Expect(conditions.ConvertToSlice()).To(HaveExactElements(
					OfType("APIServerAvailable"),
					OfType("ControlPlaneHealthy"),
					OfType("ObservabilityComponentsHealthy"),
					OfType("EveryNodeReady"),
					OfType("SystemComponentsHealthy"),
				))
			})
		})

		Describe("#ConditionTypes", func() {
			It("should return the expected condition types", func() {
				conditions := NewShootConditions(fakeClock, &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{Name: "worker"}},
						},
					},
				})

				Expect(conditions.ConditionTypes()).To(HaveExactElements(
					gardencorev1beta1.ConditionType("APIServerAvailable"),
					gardencorev1beta1.ConditionType("ControlPlaneHealthy"),
					gardencorev1beta1.ConditionType("ObservabilityComponentsHealthy"),
					gardencorev1beta1.ConditionType("EveryNodeReady"),
					gardencorev1beta1.ConditionType("SystemComponentsHealthy"),
				))
			})
		})
	})
})

func newNode(labels labels.Set, annotations map[string]string, kubeletVersion string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node1",
			Labels:      labels,
			Annotations: annotations,
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion: kubeletVersion,
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func beConditionWithStatus(status gardencorev1beta1.ConditionStatus) types.GomegaMatcher {
	return WithStatus(status)
}

func beConditionWithStatusAndMsg(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))
}
