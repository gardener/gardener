// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
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
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("health check", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		fakeClock  = testclock.NewFakeClock(time.Now())

		condition gardencorev1beta1.Condition

		controlPlaneNamespace = "shoot--foo--bar"
		kubernetesVersion     = semver.MustParse("1.27.3")

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
			workerlessDepoymentNames = []any{
				"gardener-resource-manager",
				"kube-apiserver",
				"kube-controller-manager",
			}
			commonDeploymentNames = append(workerlessDepoymentNames, "kube-scheduler", "machine-controller-manager")
		)

		tests := func(shoot *gardencorev1beta1.Shoot, names []any, isWorkerless bool) {
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
					Name:        operatingsystemconfig.KeyV1(workerPoolName1, kubernetesVersion, nil),
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
			func(k8sversion *semver.Version, nodes []corev1.Node, workerPools []gardencorev1beta1.Worker, oscSecretMeta map[string]metav1.ObjectMeta, conditionMatcher types.GomegaMatcher) {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
					*list = corev1.NodeList{Items: nodes}
					return nil
				})

				Expect(fakeClient.Create(ctx, &machinev1alpha1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
					Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(len(nodes))},
				})).To(Succeed())

				secretListOptions := []client.ListOption{
					client.InNamespace(metav1.NamespaceSystem),
					client.MatchingLabels{"gardener.cloud/role": "operating-system-config"},
				}

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), secretListOptions).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{}
					for _, m := range oscSecretMeta {
						list.Items = append(list.Items, corev1.Secret{
							ObjectMeta: m,
						})
					}
					return nil
				})

				shootObj := &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
					KubernetesVersion:     k8sversion,
				}
				shootObj.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: workerPools,
						},
					},
				})
				seedObj := &seedpkg.Seed{}
				seedObj.SetInfo(&gardencorev1beta1.Seed{})

				health := NewHealth(
					logr.Discard(),
					shootObj,
					seedObj,
					fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
					nil,
					nil,
					fakeClock,
					nil,
					nil,
				)

				exitCondition, err := health.CheckClusterNodes(ctx, fakekubernetes.NewClientSetBuilder().WithClient(c).Build(), condition)
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
				oscSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q hasn't been reported yet", nodeName)))),
			Entry("outdated OSC secret checksum for a worker pool",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, map[string]string{nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: "outdated"}, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.KeyV1(workerPoolName1, kubernetesVersion, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q is outdated", nodeName)))),
			Entry("outdated OSC secret checksum for a worker pool when shoot has not been reconciled yet",
				kubernetesVersion,
				[]corev1.Node{
					newNode(labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()}, map[string]string{nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: "outdated"}, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.KeyV1(workerPoolName1, kubernetesVersion, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q is outdated", nodeName)))),
		)
	})

	Describe("#CheckNodesScaling", func() {
		Describe("Rolling update", func() {
			It("should prioritize NodesRollOutScalingUp over NodesScalingUp when returning an error if not enough machine objects as desired were created", func() {
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentAvailable, Status: machinev1alpha1.ConditionFalse},
								{Type: machinev1alpha1.MachineDeploymentProgressing, Status: machinev1alpha1.ConditionTrue, Reason: "NewMachineSetNotAvailable"},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, []*corev1.Node{}, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("not enough machine objects created yet")))
			})

			It("should return an error if not enough machine objects as desired were created", func() {
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentProgressing, Status: machinev1alpha1.ConditionTrue, Reason: "NewMachineSetNotAvailable"}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, []*corev1.Node{}, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("not enough machine objects created yet")))
			})

			It("should return an error when detecting erroneous machines", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineUnknown},
							},
						},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}

				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentProgressing, Status: machinev1alpha1.ConditionTrue, Reason: "NewMachineSetNotAvailable"}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, []*corev1.Node{}, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("is erroneous")))
			})

			It("should return an error when not enough ready nodes are registered", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineRunning},
							},
						},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}
				nodeList := []*corev1.Node{{}}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentProgressing, Status: machinev1alpha1.ConditionTrue, Reason: "NewMachineSetNotAvailable"}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("not enough ready worker nodes registered in the cluster")))
			})

			It("should return progressing when detecting a regular node rollout (pending status)", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachinePending},
							},
						},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}
				nodeList := []*corev1.Node{{}}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentProgressing, Status: machinev1alpha1.ConditionTrue, Reason: "NewMachineSetNotAvailable"}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("provisioning and should join the cluster soon")))
			})

			It("should return progressing when detecting a regular node rollout (no status)", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace}},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}
				nodeList := []*corev1.Node{{}}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentProgressing, Status: machinev1alpha1.ConditionTrue, Reason: "NewMachineSetNotAvailable"}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("provisioning and should join the cluster soon")))
			})
		})

		Describe("Scaling up", func() {
			It("should return true if number of ready nodes equal number of desired machines", func() {
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
						},
					},
				}
				nodeList := []*corev1.Node{
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:   corev1.NodeReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal(""))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an error if not enough machine objects as desired were created", func() {
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentAvailable, Status: machinev1alpha1.ConditionFalse}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, []*corev1.Node{}, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("not enough machine objects created yet")))
			})

			It("should return an error when detecting erroneous machines", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineUnknown},
							},
						},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}

				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentAvailable, Status: machinev1alpha1.ConditionFalse}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, []*corev1.Node{}, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("is erroneous")))
			})

			It("should return an error when not enough ready nodes are registered", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineRunning},
							},
						},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}
				nodeList := []*corev1.Node{{}}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentAvailable, Status: machinev1alpha1.ConditionFalse}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("not enough ready worker nodes registered in the cluster")))
			})

			It("should return progressing when detecting a regular scale up (pending status)", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachinePending},
							},
						},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}
				nodeList := []*corev1.Node{{}}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentAvailable, Status: machinev1alpha1.ConditionFalse}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("provisioning and should join the cluster soon")))
			})

			It("should return progressing when detecting a regular scale up (no status)", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace}},
					},
				}
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}
				nodeList := []*corev1.Node{{}}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
							Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
								{Type: machinev1alpha1.MachineDeploymentAvailable, Status: machinev1alpha1.ConditionFalse}, {},
							}},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("provisioning and should join the cluster soon")))
			})
		})

		Describe("Scaling down", func() {
			It("should return true if number of registered nodes equal number of desired machines", func() {
				nodeList := []*corev1.Node{{}}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal(""))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an error if the machine for a cordoned node is not found", func() {
				nodeList := []*corev1.Node{
					{Spec: corev1.NodeSpec{Unschedulable: true}},
				}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{Items: []machinev1alpha1.MachineDeployment{}}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingDown"))
				Expect(err).To(MatchError(ContainSubstring("machine object for cordoned node \"\" not found")))
			})

			It("should return an error if the machine for a cordoned node is not deleted", func() {
				var (
					nodeName = "foo"

					machineList = &machinev1alpha1.MachineList{
						Items: []machinev1alpha1.Machine{
							{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace, Labels: map[string]string{"node": nodeName}}},
						},
					}
					nodeList = []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					}
				)
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, &machinev1alpha1.MachineDeploymentList{}, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingDown"))
				Expect(err).To(MatchError(ContainSubstring("found but corresponding machine object does not have a deletion timestamp")))
			})

			It("should return an error if there are more nodes then machines", func() {
				nodeList := []*corev1.Node{{}}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, &machinev1alpha1.MachineDeploymentList{}, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingDown"))
				Expect(err).To(MatchError(ContainSubstring("too many worker nodes are registered. Exceeding maximum desired machine count")))
			})

			It("should return progressing for a regular scale down", func() {
				var (
					nodeName = "foo"

					machineList = &machinev1alpha1.MachineList{
						Items: []machinev1alpha1.Machine{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:         "foo",
									GenerateName: "obj-",
									Namespace:    controlPlaneNamespace,
									Labels:       map[string]string{"node": nodeName},
									Finalizers:   []string{"in-deletion"},
								},
							},
						},
					}
					nodeList = []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					}
				)
				for _, machine := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machine)).To(Succeed())
					Expect(fakeClient.Delete(ctx, &machine)).To(Succeed())
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, &machinev1alpha1.MachineDeploymentList{}, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingDown"))
				Expect(err).To(MatchError(ContainSubstring("is waiting to be completely drained from pods")))
			})
		})
	})

	Describe("#CheckIfDependencyWatchdogProberScaledDownControllers", func() {
		var (
			deploymentCA  *appsv1.Deployment
			deploymentKCM *appsv1.Deployment
			deploymentMCM *appsv1.Deployment
		)

		BeforeEach(func() {
			deploymentCA = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler", Namespace: controlPlaneNamespace}, Spec: appsv1.DeploymentSpec{Replicas: ptr.To[int32](1)}}
			deploymentKCM = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: controlPlaneNamespace}, Spec: appsv1.DeploymentSpec{Replicas: ptr.To[int32](1)}}
			deploymentMCM = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: controlPlaneNamespace}, Spec: appsv1.DeploymentSpec{Replicas: ptr.To[int32](1)}}
		})

		It("should report an error because a required relevant deployment does not exist", func() {
			scaledDownDeploymentNames, err := CheckIfDependencyWatchdogProberScaledDownControllers(ctx, fakeClient, controlPlaneNamespace)
			Expect(err).To(BeNotFoundError())
			Expect(scaledDownDeploymentNames).To(BeEmpty())
		})

		It("should report nothing because all relevant deployment have replicas > 0", func() {
			Expect(fakeClient.Create(ctx, deploymentCA)).To(Succeed())
			Expect(fakeClient.Create(ctx, deploymentKCM)).To(Succeed())
			Expect(fakeClient.Create(ctx, deploymentMCM)).To(Succeed())

			scaledDownDeploymentNames, err := CheckIfDependencyWatchdogProberScaledDownControllers(ctx, fakeClient, controlPlaneNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(scaledDownDeploymentNames).To(BeEmpty())
		})

		It("should report names because some relevant deployment have replicas == 0", func() {
			deploymentKCM.Spec.Replicas = nil
			deploymentMCM.Spec.Replicas = ptr.To[int32](0)

			Expect(fakeClient.Create(ctx, deploymentCA)).To(Succeed())
			Expect(fakeClient.Create(ctx, deploymentKCM)).To(Succeed())
			Expect(fakeClient.Create(ctx, deploymentMCM)).To(Succeed())

			scaledDownDeploymentNames, err := CheckIfDependencyWatchdogProberScaledDownControllers(ctx, fakeClient, controlPlaneNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(scaledDownDeploymentNames).To(HaveExactElements(deploymentKCM.Name, deploymentMCM.Name))
		})
	})

	Describe("#CheckForExpiredNodeLeases", func() {
		var (
			nodeName = "node1"

			node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}

			validLease = &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: "kube-node-lease",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To[int32](40),
				},
			}

			expiredLease = &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: "kube-node-lease",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To(int32(-40)),
				},
			}

			unrelatedLease = &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node2",
					Namespace: "kube-node-lease",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To[int32](40),
				},
			}
		)

		DescribeTable("#CheckForExpiredNodeLeases",
			func(lease *coordinationv1.Lease, node *corev1.Node, additionalNodeNames []string, expected types.GomegaMatcher) {
				leaseList := coordinationv1.LeaseList{}
				if lease != nil {
					leaseList.Items = append(leaseList.Items, *lease)
				}

				nodeList := corev1.NodeList{}
				if node != nil {
					nodeList.Items = append(nodeList.Items, *node)
				}

				for _, nodeName := range additionalNodeNames {
					nodeList.Items = append(nodeList.Items, corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}})

					lease := validLease.DeepCopy()
					lease.Name = nodeName
					leaseList.Items = append(leaseList.Items, *lease)
				}

				Expect(CheckForExpiredNodeLeases(&nodeList, &leaseList, fakeClock)).To(expected)
			},

			Entry("should return nil if there is an unexpired lease for node", validLease, node, nil, BeNil()),
			Entry("should return nil if no leases are present", nil, node, nil, BeNil()),
			Entry("should return nil if no nodes are present", validLease, nil, nil, BeNil()),
			Entry("should return nil if no node could be found for the lease", unrelatedLease, node, nil, BeNil()),
			Entry("should return nil if less than 20% of leases are expired", expiredLease, node, []string{"node2", "node3", "node4", "node5", "node6"}, BeNil()),
			Entry("should return an error if exactly 20% of leases are expired", expiredLease, node, []string{"node2", "node3", "node4", "node5"}, MatchError(ContainSubstring("Leases in kube-node-lease namespace are expired"))),
			Entry("should return an error if at least 20% of leases are expired", expiredLease, node, nil, MatchError(ContainSubstring("Leases in kube-node-lease namespace are expired"))),
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
					LeaseDurationSeconds: ptr.To[int32](40),
				},
			}

			expiredLease = coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-node-agent-node1",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To(int32(-40)),
				},
			}

			unrelatedLease = coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-node-agent-node2",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: ptr.To[int32](40),
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
			Entry("should return Error that node agent is not running if no matching lease could be found for node", unrelatedLease, MatchError(ContainSubstring("not running"))),
			Entry("should return Error that node agent stopped running if the lease for the node is not valid anymore", expiredLease, MatchError(ContainSubstring("stopped running"))),
		)
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
