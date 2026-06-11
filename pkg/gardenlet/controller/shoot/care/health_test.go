// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	fakerestclient "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/testing"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("health check", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		fakeClock  = testclock.NewFakeClock(time.Now())

		condition gardencorev1beta1.Condition

		controlPlaneNamespace = "shoot--foo--bar"
		kubernetesVersion     = semver.MustParse("1.33.3")

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
		// Starting with controller-runtime v0.22.0, the default object tracker does not work with resources which include
		// structs directly as pointer, e.g. *MachineConfiguration in Machine resource. Hence, use the old one instead.
		objectTracker := testing.NewObjectTracker(kubernetes.SeedScheme, scheme.Codecs.UniversalDecoder())
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjectTracker(objectTracker).Build()
		fakeClock = testclock.NewFakeClock(time.Now())
		condition = gardencorev1beta1.Condition{
			Type:               "test",
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
	})

	Describe("#ComputeRequiredControlPlaneDeployments", func() {
		var (
			workerlessDeploymentNames = []any{
				"gardener-resource-manager",
				"kube-apiserver",
				"kube-controller-manager",
			}
			commonDeploymentNames = append(workerlessDeploymentNames, "kube-scheduler", "machine-controller-manager")
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
			tests(workerlessShoot, workerlessDeploymentNames, true)
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
			workerPoolName1            = "cpu-worker-1"
			workerPoolName2            = "cpu-worker-2"
			cloudConfigSecretChecksum1 = "foo"
			cloudConfigSecretChecksum2 = "foo"
			nodeName                   = "node1"
			oscSecretMeta              = map[string]metav1.ObjectMeta{
				workerPoolName1: {
					Name:        operatingsystemconfig.Key(kubernetesVersion, nil, &gardencorev1beta1.Worker{Name: workerPoolName1}, false, nil, nil),
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

		DescribeTable("#CheckClusterNodes",
			func(k8sversion *semver.Version, nodes []corev1.Node, workerPools []gardencorev1beta1.Worker, oscSecretMeta map[string]metav1.ObjectMeta, desiredMachines int32, leases []coordinationv1.Lease, conditionMatcher types.GomegaMatcher) {
				// Create a fake shoot client populated with the nodes, OSC secrets, and leases for this test entry
				fakeShootClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

				for i := range nodes {
					node := nodes[i]
					Expect(fakeShootClient.Create(ctx, &node)).To(Succeed())
				}
				for _, meta := range oscSecretMeta {
					// Add the role label needed for the List filter
					if meta.Labels == nil {
						meta.Labels = map[string]string{}
					} else {
						meta.Labels = maps.Clone(meta.Labels)
					}
					meta.Labels["gardener.cloud/role"] = "operating-system-config"
					meta.Namespace = metav1.NamespaceSystem
					Expect(fakeShootClient.Create(ctx, &corev1.Secret{ObjectMeta: meta})).To(Succeed())
				}
				for i := range leases {
					lease := leases[i]
					if lease.Namespace == "" {
						lease.Namespace = metav1.NamespaceSystem
					}
					Expect(fakeShootClient.Create(ctx, &lease)).To(Succeed())
				}

				if desiredMachines == 0 {
					desiredMachines = int32(len(nodes))
				}
				Expect(fakeClient.Create(ctx, &machinev1alpha1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
					Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: desiredMachines},
				})).To(Succeed())

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
				seedObj.SetInfo(&gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Settings: &gardencorev1beta1.SeedSettings{
							DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{
								Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{Enabled: false},
							},
						},
					},
				})

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

				exitCondition, err := health.CheckClusterNodes(ctx, fakekubernetes.NewClientSetBuilder().WithClient(fakeShootClient).Build(), condition)
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
				int32(0),
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
				int32(0),
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
				int32(0),
				nil,
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
				int32(0),
				nil,
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
						Name:        operatingsystemconfig.Key(kubernetesVersion, nil, &gardencorev1beta1.Worker{Name: workerPoolName1}, false, nil, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				int32(0),
				nil,
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
						Name:        operatingsystemconfig.Key(kubernetesVersion, nil, &gardencorev1beta1.Worker{Name: workerPoolName1}, false, nil, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				int32(0),
				nil,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "OperatingSystemConfigOutdated", fmt.Sprintf("the last successfully applied operating system config on node %q is outdated", nodeName)))),
			Entry("should not report NodeAgentUnhealthy for nodes not managed by MCM",
				kubernetesVersion,
				[]corev1.Node{
					newNode(
						labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()},
						map[string]string{nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: cloudConfigSecretChecksum1},
						kubernetesVersion.Original(),
					),
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "non-mcm-node",
							Annotations: map[string]string{
								"node.machine.sapcloud.io/not-managed-by-mcm": "1",
							},
						},
					},
				},
				[]gardencorev1beta1.Worker{{Name: workerPoolName1, Maximum: 10, Minimum: 1}},
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.Key(kubernetesVersion, nil, &gardencorev1beta1.Worker{Name: workerPoolName1}, false, nil, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				int32(1),
				[]coordinationv1.Lease{{
					ObjectMeta: metav1.ObjectMeta{Name: "gardener-node-agent-" + nodeName},
					Spec:       coordinationv1.LeaseSpec{RenewTime: &metav1.MicroTime{Time: time.Now()}, LeaseDurationSeconds: new(int32(40))},
				}},
				BeNil()),
			Entry("should report NodeAgentUnhealthy for managed node without lease",
				kubernetesVersion,
				[]corev1.Node{
					newNode(
						labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()},
						map[string]string{nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: cloudConfigSecretChecksum1},
						kubernetesVersion.Original(),
					),
				},
				[]gardencorev1beta1.Worker{{Name: workerPoolName1, Maximum: 10, Minimum: 1}},
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.Key(kubernetesVersion, nil, &gardencorev1beta1.Worker{Name: workerPoolName1}, false, nil, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				int32(0),
				[]coordinationv1.Lease{},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "NodeAgentUnhealthy", fmt.Sprintf("gardener-node-agent is not running on node %q", nodeName)))),
			Entry("only preserved-failed node is unhealthy — returns preserved failure",
				kubernetesVersion,
				[]corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "preserved-node",
							Labels: labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()},
							Annotations: map[string]string{
								nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: cloudConfigSecretChecksum1,
							},
						},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{KubeletVersion: kubernetesVersion.Original()},
							Conditions: []corev1.NodeCondition{
								{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
								{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "KubeletNotReady"},
							},
						},
					},
				},
				[]gardencorev1beta1.Worker{{Name: workerPoolName1, Maximum: 10, Minimum: 1}},
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.KeyV1(workerPoolName1, kubernetesVersion, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				int32(0),
				[]coordinationv1.Lease{},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "NodeUnhealthy", "node and backing machine preserved by MCM"))),
			Entry("unpreserved failure takes priority over preserved failure",
				kubernetesVersion,
				[]corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "preserved-node",
							Labels: labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()},
							Annotations: map[string]string{
								nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: cloudConfigSecretChecksum1,
							},
						},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{KubeletVersion: kubernetesVersion.Original()},
							Conditions: []corev1.NodeCondition{
								{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
								{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "KubeletNotReady"},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "unpreserved-node",
							Labels: labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": kubernetesVersion.Original()},
							Annotations: map[string]string{
								nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig: cloudConfigSecretChecksum1,
							},
						},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{KubeletVersion: kubernetesVersion.Original()},
							Conditions: []corev1.NodeCondition{
								{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "KubeletNotReady"},
							},
						},
					},
				},
				[]gardencorev1beta1.Worker{{Name: workerPoolName1, Maximum: 10, Minimum: 2}},
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        operatingsystemconfig.KeyV1(workerPoolName1, kubernetesVersion, nil),
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				int32(2),
				[]coordinationv1.Lease{},
				PointTo(And(
					beConditionWithStatus(gardencorev1beta1.ConditionFalse),
					WithReason("NodeUnhealthy"),
					WithMessageSubstrings(`Node "unpreserved-node"`),
				))),
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

			It("should return an error when not enough nodes are registered", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: controlPlaneNamespace},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineRunning},
							},
						},
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
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(2)},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("not enough ready worker nodes registered in the cluster")))
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
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("not enough ready worker nodes registered in the cluster")))
			})

			It("should return an error when the only registered node is ready but unschedulable (cordoned)", func() {
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
				// One node is registered and Ready but cordoned (unschedulable) - it must NOT count as ready+schedulable.
				nodeList := []*corev1.Node{
					{
						Spec: corev1.NodeSpec{Unschedulable: true},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
							},
						},
					},
				}
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(1)},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesRollOutScalingUp"))
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
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(2)},
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
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(2)},
						},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal("NodesScalingUp"))
				Expect(err).To(MatchError(ContainSubstring("provisioning and should join the cluster soon")))
			})
		})

		Describe("Scaling down", func() {
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

			It("should not report a scale-down error when a preserved failed node is included in the node list and registered nodes equal desired machines", func() {
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: "obj-",
								Namespace:    controlPlaneNamespace,
								Labels:       map[string]string{"node": "preserved-node"},
							},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{
									Phase: machinev1alpha1.MachineFailed,
									PreserveExpiryTime: &metav1.Time{
										Time: time.Now().Add(10 * time.Minute),
									},
								},
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
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(3)},
						},
					},
				}
				nodeList := []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
						Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
						Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						}},
					},
					{
						// preserved failed node — cordoned by MCM, included in nodesForScalingCheck
						ObjectMeta: metav1.ObjectMeta{Name: "preserved-node"},
						Spec:       corev1.NodeSpec{Unschedulable: true},
						Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
							{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
							{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
						}},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
				Expect(msg).To(Equal(""))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report draining for an unpreserved cordoned node even when a preserved cordoned node is also present", func() {
				var (
					preservedNodeName   = "preserved-node"
					unpreservedNodeName = "draining-node"
				)
				machineList := &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:         "preserved-machine",
								GenerateName: "obj-",
								Namespace:    controlPlaneNamespace,
								Labels:       map[string]string{"node": preservedNodeName},
							},
							Status: machinev1alpha1.MachineStatus{
								CurrentStatus: machinev1alpha1.CurrentStatus{
									Phase: machinev1alpha1.MachineFailed,
									PreserveExpiryTime: &metav1.Time{
										Time: time.Now().Add(10 * time.Minute),
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:         "draining-machine",
								GenerateName: "obj-",
								Namespace:    controlPlaneNamespace,
								Labels:       map[string]string{"node": unpreservedNodeName},
								Finalizers:   []string{"in-deletion"},
							},
						},
					},
				}
				for i := range machineList.Items {
					Expect(fakeClient.Create(ctx, &machineList.Items[i])).To(Succeed())
				}
				Expect(fakeClient.Delete(ctx, &machineList.Items[1])).To(Succeed())

				// MCD scaled down to 2: 1 running + 1 preserved. The unpreserved node is draining (registeredNodes=3 > desiredMachines=2).
				machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
					Items: []machinev1alpha1.MachineDeployment{
						{
							ObjectMeta: metav1.ObjectMeta{GenerateName: "deploy", Namespace: controlPlaneNamespace},
							Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: int32(2)},
						},
					},
				}
				nodeList := []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
						Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: preservedNodeName},
						Spec:       corev1.NodeSpec{Unschedulable: true},
						Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
							{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
							{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
						}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: unpreservedNodeName},
						Spec:       corev1.NodeSpec{Unschedulable: true},
						Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						}},
					},
				}
				msg, err := CheckNodesScaling(ctx, fakeClient, nodeList, machineDeploymentList, controlPlaneNamespace)
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
			deploymentCA = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler", Namespace: controlPlaneNamespace}, Spec: appsv1.DeploymentSpec{Replicas: new(int32(1))}}
			deploymentKCM = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: controlPlaneNamespace}, Spec: appsv1.DeploymentSpec{Replicas: new(int32(1))}}
			deploymentMCM = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: controlPlaneNamespace}, Spec: appsv1.DeploymentSpec{Replicas: new(int32(1))}}
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

		It("should report names because some relevant deployment have replicas == 0 and meltdown annotation is set", func() {
			deploymentKCM.Spec.Replicas = nil
			deploymentMCM.Spec.Replicas = new(int32(0))

			// Set meltdown annotation only on deploymentMCM
			if deploymentMCM.Annotations == nil {
				deploymentMCM.Annotations = map[string]string{}
			}
			deploymentMCM.Annotations["dependency-watchdog.gardener.cloud/meltdown-protection-active"] = ""

			Expect(fakeClient.Create(ctx, deploymentCA)).To(Succeed())
			Expect(fakeClient.Create(ctx, deploymentKCM)).To(Succeed())
			Expect(fakeClient.Create(ctx, deploymentMCM)).To(Succeed())

			scaledDownDeploymentNames, err := CheckIfDependencyWatchdogProberScaledDownControllers(ctx, fakeClient, controlPlaneNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(scaledDownDeploymentNames).To(HaveExactElements(deploymentMCM.Name))
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
					LeaseDurationSeconds: new(int32(40)),
				},
			}

			expiredLease = &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: "kube-node-lease",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: new(int32(-40)),
				},
			}

			unrelatedLease = &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node2",
					Namespace: "kube-node-lease",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: new(int32(40)),
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
					LeaseDurationSeconds: new(int32(40)),
				},
			}

			expiredLease = coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-node-agent-node1",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: new(int32(-40)),
				},
			}

			unrelatedLease = coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-node-agent-node2",
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &metav1.MicroTime{Time: fakeClock.Now()},
					LeaseDurationSeconds: new(int32(40)),
				},
			}

			nodeList = []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
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

			Expect(CheckNodeAgentLeases(nodeList, &leaseList, fakeClock)).To(expected)
		},
			Entry("should return nil if there is a matching lease for node", validLease, BeNil()),
			Entry("should return Error that node agent is not running if no matching lease could be found for node", unrelatedLease, MatchError(ContainSubstring("not running"))),
			Entry("should return Error that node agent stopped running if the lease for the node is not valid anymore", expiredLease, MatchError(ContainSubstring("stopped running"))),
		)
	})

	Describe("#CheckSystemdUnitsReady", func() {
		It("should return nil when no nodes have the condition", func() {
			nodes := []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
			}

			Expect(CheckSystemdUnitsReady(nodes)).To(Succeed())
		})

		It("should return nil when all nodes report healthy systemd units", func() {
			nodes := []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   "SystemdUnitsReady",
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}

			Expect(CheckSystemdUnitsReady(nodes)).To(Succeed())
		})

		It("should return error when a node reports unhealthy systemd units", func() {
			nodes := []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:    "SystemdUnitsReady",
								Status:  corev1.ConditionFalse,
								Message: "bad.service: failed",
							},
						},
					},
				},
			}

			Expect(CheckSystemdUnitsReady(nodes)).To(MatchError(ContainSubstring("bad.service: failed")))
		})

		It("should return error when a node reports unknown systemd units status", func() {
			nodes := []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:    "SystemdUnitsReady",
								Status:  corev1.ConditionUnknown,
								Message: "unable to determine status",
							},
						},
					},
				},
			}

			Expect(CheckSystemdUnitsReady(nodes)).To(MatchError(ContainSubstring("unable to determine status")))
		})

		It("should aggregate all unhealthy nodes", func() {
			nodes := []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:    "SystemdUnitsReady",
								Status:  corev1.ConditionFalse,
								Message: "bad.service: failed",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:    "SystemdUnitsReady",
								Status:  corev1.ConditionFalse,
								Message: "other.service: inactive but should be enabled",
							},
						},
					},
				},
			}

			err := CheckSystemdUnitsReady(nodes)
			Expect(err).To(MatchError(ContainSubstring("node1")))
			Expect(err).To(MatchError(ContainSubstring("node2")))
		})
	})

	Describe("#CheckPreservation", func() {
		var (
			health        *Health
			preservedCond gardencorev1beta1.Condition
		)

		BeforeEach(func() {
			shootObj := &shootpkg.Shoot{
				ControlPlaneNamespace: controlPlaneNamespace,
				KubernetesVersion:     kubernetesVersion,
			}
			shootObj.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{Name: "worker"}},
					},
				},
			})
			seedObj := &seedpkg.Seed{}
			seedObj.SetInfo(&gardencorev1beta1.Seed{})

			health = NewHealth(
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

			preservedCond = gardencorev1beta1.Condition{
				Type:               gardencorev1beta1.ShootNoPreservedFailedMachines,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			}
		})

		It("should set condition to True when no MachineDeployments have preserved failed machines", func() {
			machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
				Items: []machinev1alpha1.MachineDeployment{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "deploy-1", Namespace: controlPlaneNamespace},
						Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 0},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "deploy-2", Namespace: controlPlaneNamespace},
						Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 0},
					},
				},
			}

			result := health.CheckPreservation(machineDeploymentList, preservedCond)

			Expect(result).NotTo(BeNil())
			Expect(*result).To(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionTrue, "NoFailedMachinesPreserved", "No failed machines are being preserved."))
		})

		It("should set condition to False when one MachineDeployment has preserved failed machines", func() {
			machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
				Items: []machinev1alpha1.MachineDeployment{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "deploy-1", Namespace: controlPlaneNamespace},
						Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 2},
					},
				},
			}

			result := health.CheckPreservation(machineDeploymentList, preservedCond)

			Expect(result).NotTo(BeNil())
			Expect(*result).To(beConditionWithStatusAndMsg(
				gardencorev1beta1.ConditionFalse,
				"FailedMachinesPreserved",
				"Cluster has 2 preserved failed machine(s).",
			))
		})

		It("should set condition to False when multiple MachineDeployments have preserved failed machines", func() {
			machineDeploymentList := &machinev1alpha1.MachineDeploymentList{
				Items: []machinev1alpha1.MachineDeployment{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "deploy-1", Namespace: controlPlaneNamespace},
						Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 1},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "deploy-2", Namespace: controlPlaneNamespace},
						Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 3},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "deploy-3", Namespace: controlPlaneNamespace},
						Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 0},
					},
				},
			}

			result := health.CheckPreservation(machineDeploymentList, preservedCond)

			Expect(result).NotTo(BeNil())
			Expect(*result).To(beConditionWithStatusAndMsg(
				gardencorev1beta1.ConditionFalse,
				"FailedMachinesPreserved",
				"Cluster has 4 preserved failed machine(s).",
			))
		})

		It("should set condition to True when MachineDeploymentList is empty", func() {
			machineDeploymentList := &machinev1alpha1.MachineDeploymentList{}

			result := health.CheckPreservation(machineDeploymentList, preservedCond)

			Expect(result).NotTo(BeNil())
			Expect(*result).To(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionTrue, "NoFailedMachinesPreserved", "No failed machines are being preserved."))
		})
	})

	Describe("#checkSystemComponents DaemonSet suppression", func() {
		// These tests verify that checkSystemComponents suppresses SystemComponentsHealthy=False
		// when the only unhealthy DaemonSet pods are on preserved-failed-machine nodes.

		var (
			fakeShootClient client.Client

			makeHealthAndShoot = func(noPreservedStatus gardencorev1beta1.ConditionStatus) (*Health, *gardencorev1beta1.Shoot) {
				shootInfo := &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						// CredentialsBindingName makes HasManagedInfrastructure return true, which
						// ensures noPreservedFailedMachines is initialized (required for these tests).
						// ControlPlane on the worker makes IsSelfHosted return true, which skips the
						// VPN tunnel check — avoiding the need to import the botanist package just to
						// stub SetupPortForwarder. This combination is artificial but used as a test fixture.
						CredentialsBindingName: new("test-binding"),
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{
								Name:         "worker",
								ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
							}},
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type:  gardencorev1beta1.LastOperationTypeReconcile,
							State: gardencorev1beta1.LastOperationStateSucceeded,
						},
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   gardencorev1beta1.ShootNoPreservedFailedMachines,
								Status: noPreservedStatus,
							},
						},
					},
				}
				shootObj := &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
					KubernetesVersion:     kubernetesVersion,
				}
				shootObj.SetInfo(shootInfo)
				seedObj := &seedpkg.Seed{}
				seedObj.SetInfo(&gardencorev1beta1.Seed{})

				fakeREST := &fakerestclient.RESTClient{
					NegotiatedSerializer: serializer.NewCodecFactory(kubernetes.ShootScheme).WithoutConversion(),
					Resp: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("")),
					},
				}
				h := NewHealth(
					logr.Discard(),
					shootObj,
					seedObj,
					fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
					fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build(),
					func() (kubernetes.Interface, bool, error) {
						return fakekubernetes.NewClientSetBuilder().WithClient(fakeShootClient).WithRESTClient(fakeREST).Build(), true, nil
					},
					fakeClock,
					&gardenletconfigv1alpha1.GardenletConfiguration{},
					nil,
				)
				return h, shootInfo
			}
		)

		BeforeEach(func() {
			// Seed client: a ManagedResource with ResourcesHealthy=False referencing a DaemonSet.
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: "test-mr", Namespace: controlPlaneNamespace},
				Spec:       resourcesv1alpha1.ManagedResourceSpec{SecretRefs: []corev1.LocalObjectReference{{Name: "dummy"}}},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
						{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionFalse, Message: "DaemonSet kube-system/csi-driver is unhealthy"},
						{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse},
					},
					Resources: []resourcesv1alpha1.ObjectReference{
						{ObjectReference: corev1.ObjectReference{Kind: "DaemonSet", Namespace: "kube-system", Name: "csi-driver"}},
					},
				},
			}
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			// Seed client: a MachineDeployment with preserved failed replicas so that
			// CheckPreservation keeps noPreservedFailedMachines as False (mirrors the shoot status).
			md := &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: controlPlaneNamespace},
				Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: 3},
				Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 1},
			}
			Expect(fakeClient.Create(ctx, md)).To(Succeed())

			// Shoot client: unhealthy DaemonSet, preserved node, not-ready pod on that node,
			// and a tunnel pod so the tunnel check does not fail.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "csi-driver", Namespace: "kube-system"},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "csi-driver"}},
					UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
						Type:          appsv1.RollingUpdateDaemonSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateDaemonSet{MaxUnavailable: new(intstr.FromInt32(1))},
					},
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					CurrentNumberScheduled: 3,
					UpdatedNumberScheduled: 3,
					NumberAvailable:        2,
					NumberUnavailable:      1,
				},
			}
			tunnelPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "vpn-shoot", Namespace: metav1.NamespaceSystem, Labels: map[string]string{"type": "tunnel"}},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			}
			preservedNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "preserved-node"},
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
					{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				}},
			}
			notReadyPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "csi-driver-preserved", Namespace: "kube-system", Labels: map[string]string{"app": "csi-driver"}},
				Spec:       corev1.PodSpec{NodeName: "preserved-node"},
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionFalse},
				}},
			}

			fakeShootClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
				WithObjects(ds, tunnelPod, preservedNode, notReadyPod).
				Build()
		})

		It("suppresses SystemComponentsHealthy=False when all DaemonSet pods failing are on preserved nodes", func() {
			h, shootInfo := makeHealthAndShoot(gardencorev1beta1.ConditionFalse) // preserved machines exist
			conditions := NewShootConditions(fakeClock, shootInfo)
			resultConditions := h.Check(ctx, nil, conditions)

			var systemComponentsCond *gardencorev1beta1.Condition
			for i := range resultConditions {
				if resultConditions[i].Type == gardencorev1beta1.ShootSystemComponentsHealthy {
					systemComponentsCond = &resultConditions[i]
					break
				}
			}
			Expect(systemComponentsCond).NotTo(BeNil())
			Expect(systemComponentsCond.Status).To(Equal(gardencorev1beta1.ConditionTrue))
		})

		It("does not suppress when ShootNoPreservedFailedMachines is True", func() {
			// Replace the BeforeEach MachineDeployment (PreservedFailedReplicas=1) with one that
			// has no preserved replicas, so CheckPreservation keeps the condition True.
			md := &machinev1alpha1.MachineDeployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: controlPlaneNamespace}}
			Expect(fakeClient.Delete(ctx, md)).To(Succeed())
			Expect(fakeClient.Create(ctx, &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: controlPlaneNamespace},
				Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: 3},
				Status:     machinev1alpha1.MachineDeploymentStatus{PreservedFailedReplicas: 0},
			})).To(Succeed())

			h, shootInfo := makeHealthAndShoot(gardencorev1beta1.ConditionTrue) // no preserved machines
			conditions := NewShootConditions(fakeClock, shootInfo)
			resultConditions := h.Check(ctx, nil, conditions)

			var systemComponentsCond *gardencorev1beta1.Condition
			for i := range resultConditions {
				if resultConditions[i].Type == gardencorev1beta1.ShootSystemComponentsHealthy {
					systemComponentsCond = &resultConditions[i]
					break
				}
			}
			Expect(systemComponentsCond).NotTo(BeNil())
			Expect(systemComponentsCond.Status).To(Equal(gardencorev1beta1.ConditionFalse))
		})

		It("does not suppress when the unhealthy pod is on a non-preserved node", func() {
			// Replace the preserved node with a normal node.
			normalNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "preserved-node"}} // same name, no condition
			oneUnavailable := intstr.FromInt32(1)
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "csi-driver", Namespace: "kube-system"},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "csi-driver"}},
					UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
						Type:          appsv1.RollingUpdateDaemonSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateDaemonSet{MaxUnavailable: &oneUnavailable},
					},
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					CurrentNumberScheduled: 3,
					UpdatedNumberScheduled: 3,
					NumberAvailable:        2,
					NumberUnavailable:      1,
				},
			}
			tunnelPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "vpn-shoot", Namespace: metav1.NamespaceSystem, Labels: map[string]string{"type": "tunnel"}},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			}
			notReadyPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "csi-driver-normal", Namespace: "kube-system", Labels: map[string]string{"app": "csi-driver"}},
				Spec:       corev1.PodSpec{NodeName: "preserved-node"},
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionFalse},
				}},
			}
			fakeShootClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
				WithObjects(ds, tunnelPod, normalNode, notReadyPod).
				Build()

			h, shootInfo := makeHealthAndShoot(gardencorev1beta1.ConditionFalse)
			conditions := NewShootConditions(fakeClock, shootInfo)
			resultConditions := h.Check(ctx, nil, conditions)

			var systemComponentsCond *gardencorev1beta1.Condition
			for i := range resultConditions {
				if resultConditions[i].Type == gardencorev1beta1.ShootSystemComponentsHealthy {
					systemComponentsCond = &resultConditions[i]
					break
				}
			}
			Expect(systemComponentsCond).NotTo(BeNil())
			Expect(systemComponentsCond.Status).To(Equal(gardencorev1beta1.ConditionFalse))
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
					OfType("NoPreservedFailedMachines"),
					OfType("SystemComponentsHealthy"),
				))
			})

			It("should exclude NoPreservedFailedMachines when status is True", func() {
				conditions := NewShootConditions(fakeClock, &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{Name: "worker"}},
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: gardencorev1beta1.ShootNoPreservedFailedMachines, Status: gardencorev1beta1.ConditionTrue},
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
					gardencorev1beta1.ConditionType("NoPreservedFailedMachines"),
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
