// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("DaemonSet", func() {
	oneUnavailable := intstr.FromInt32(1)

	DescribeTable("#CheckDaemonSet",
		func(daemonSet *appsv1.DaemonSet, matcher types.GomegaMatcher) {
			err := health.CheckDaemonSet(daemonSet)
			Expect(err).To(matcher)
		},
		Entry("not observed at latest version", &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough scheduled", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
		}, HaveOccurred()),
		Entry("misscheduled pods", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{NumberMisscheduled: 1},
		}, HaveOccurred()),
		Entry("too many unavailable pods during rollout", &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &oneUnavailable,
				},
			}},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3,
				CurrentNumberScheduled: 3,
				NumberUnavailable:      2,
				NumberReady:            1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 2,
			},
		}, HaveOccurred()),
		Entry("too many unavailable pods", &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &oneUnavailable,
				},
			}},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 2,
				CurrentNumberScheduled: 2,
				NumberUnavailable:      2,
				NumberReady:            0,
			},
		}, HaveOccurred()),
		Entry("healthy", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				CurrentNumberScheduled: 1,
				NumberReady:            1,
			},
		}, BeNil()),
		Entry("healthy with allowed unavailable pods during rollout", &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &oneUnavailable,
				},
			}},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3,
				CurrentNumberScheduled: 3,
				NumberUnavailable:      1,
				NumberReady:            2,
				NumberAvailable:        2,
				UpdatedNumberScheduled: 1,
			},
		}, BeNil()),
	)

	Describe("IsDaemonSetProgressing", func() {
		var (
			daemonSet *appsv1.DaemonSet
		)

		BeforeEach(func() {
			daemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Status: appsv1.DaemonSetStatus{
					ObservedGeneration:     42,
					DesiredNumberScheduled: 3,
					UpdatedNumberScheduled: 3,
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsDaemonSetProgressing(daemonSet)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("DaemonSet is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			daemonSet.Status.ObservedGeneration--

			progressing, reason := health.IsDaemonSetProgressing(daemonSet)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if replicas still need to be updated", func() {
			daemonSet.Status.UpdatedNumberScheduled--

			progressing, reason := health.IsDaemonSetProgressing(daemonSet)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("2 of 3 replica(s) have been updated"))
		})
	})

	Describe("#CheckDaemonSetWithPreservedNodes", func() {
		var (
			ctx context.Context

			ds *appsv1.DaemonSet
		)

		BeforeEach(func() {
			ctx = context.Background()
			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ds",
					Namespace: "kube-system",
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
						Type: appsv1.RollingUpdateDaemonSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateDaemonSet{
							MaxUnavailable: func() *intstr.IntOrString { v := intstr.FromInt32(1); return &v }(),
						},
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
		})

		preservedNode := func(name string) *corev1.Node {
			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			}
		}

		normalNode := func(name string) *corev1.Node {
			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: name},
			}
		}

		notReadyPod := func(name, nodeName string) *corev1.Pod {
			return &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "kube-system",
					Labels:    map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionFalse},
					},
				},
			}
		}

		It("returns false when DaemonSet is already healthy", func() {
			ds.Status.NumberUnavailable = 0
			ds.Status.NumberAvailable = 3

			c := fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
			preservedNodeNames, err := health.GetPreservedNodeNames(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			suppressed, err := health.CheckDaemonSetWithPreservedNodes(ctx, c, ds, preservedNodeNames)
			Expect(err).NotTo(HaveOccurred())
			Expect(suppressed).To(BeFalse())
		})

		It("returns false when NumberAvailable is zero (real outage)", func() {
			ds.Status.NumberAvailable = 0
			ds.Status.NumberUnavailable = 3

			c := fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
			preservedNodeNames, err := health.GetPreservedNodeNames(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			suppressed, err := health.CheckDaemonSetWithPreservedNodes(ctx, c, ds, preservedNodeNames)
			Expect(err).NotTo(HaveOccurred())
			Expect(suppressed).To(BeFalse())
		})

		It("returns false when no preserved nodes exist", func() {
			c := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
				WithObjects(normalNode("node-1"), normalNode("node-2")).
				Build()

			preservedNodeNames, err := health.GetPreservedNodeNames(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			suppressed, err := health.CheckDaemonSetWithPreservedNodes(ctx, c, ds, preservedNodeNames)
			Expect(err).NotTo(HaveOccurred())
			Expect(suppressed).To(BeFalse())
		})

		It("returns true when the only unavailable pod is on a preserved node", func() {
			c := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
				WithObjects(
					preservedNode("node-preserved"),
					notReadyPod("ds-pod-preserved", "node-preserved"),
				).
				Build()

			preservedNodeNames, err := health.GetPreservedNodeNames(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			suppressed, err := health.CheckDaemonSetWithPreservedNodes(ctx, c, ds, preservedNodeNames)
			Expect(err).NotTo(HaveOccurred())
			Expect(suppressed).To(BeTrue())
		})

		It("returns false when unavailable pods are on both preserved and normal nodes", func() {
			ds.Status.NumberUnavailable = 2
			ds.Status.NumberAvailable = 1

			c := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
				WithObjects(
					preservedNode("node-preserved"),
					normalNode("node-normal"),
					notReadyPod("ds-pod-preserved", "node-preserved"),
					notReadyPod("ds-pod-normal", "node-normal"),
				).
				Build()

			preservedNodeNames, err := health.GetPreservedNodeNames(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			suppressed, err := health.CheckDaemonSetWithPreservedNodes(ctx, c, ds, preservedNodeNames)
			Expect(err).NotTo(HaveOccurred())
			Expect(suppressed).To(BeFalse())
		})

		It("returns false when adjusted unavailable count still exceeds maxUnavailable during rollout", func() {
			// 2 unavailable, 1 on preserved node, maxUnavailable=1 → adjusted=1 which equals maxUnavailable, so suppressed
			// But if adjusted > maxUnavailable it should NOT be suppressed
			ds.Status.UpdatedNumberScheduled = 2 // rollout in progress
			ds.Status.NumberUnavailable = 3
			ds.Status.NumberAvailable = 0
			// NumberAvailable=0 → early exit, not suppressed
			c := fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
			preservedNodeNames, err := health.GetPreservedNodeNames(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			suppressed, err := health.CheckDaemonSetWithPreservedNodes(ctx, c, ds, preservedNodeNames)
			Expect(err).NotTo(HaveOccurred())
			Expect(suppressed).To(BeFalse())
		})

		It("returns true when adjusted unavailable count is within maxUnavailable during rollout", func() {
			// Rollout in progress: 2 unavailable exceeds maxUnavailable=1, but 1 of the unavailable
			// pods is on a preserved node → adjusted=1, which does not exceed maxUnavailable=1 → suppressed.
			ds.Status.UpdatedNumberScheduled = 2 // rollout in progress
			ds.Status.NumberUnavailable = 2
			ds.Status.NumberAvailable = 1

			c := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
				WithObjects(
					preservedNode("node-preserved"),
					notReadyPod("ds-pod-preserved", "node-preserved"),
				).
				Build()

			preservedNodeNames, err := health.GetPreservedNodeNames(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			suppressed, err := health.CheckDaemonSetWithPreservedNodes(ctx, c, ds, preservedNodeNames)
			Expect(err).NotTo(HaveOccurred())
			Expect(suppressed).To(BeTrue())
		})
	})
})
