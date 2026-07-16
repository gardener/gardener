// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate_test

import (
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/inplaceupdate"
)

var poolName = "worker-pool"

var _ = Describe("Reconciler", func() {
	Describe("#ShouldSkipPod", func() {
		It("should skip succeeded pods", func() {
			Expect(ShouldSkipPod(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}})).To(BeTrue())
		})

		It("should skip failed pods", func() {
			Expect(ShouldSkipPod(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}})).To(BeTrue())
		})

		It("should skip mirror pods", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{corev1.MirrorPodAnnotationKey: "abc"},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip DaemonSet owned pods", func() {
			isController := true
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{
					Kind:       "DaemonSet",
					APIVersion: "apps/v1",
					Name:       "ds",
					Controller: &isController,
				}},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip gardenlet pods", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1beta1constants.LabelRole: "gardenlet"},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip gardener-resource-manager pods", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip pods that tolerate the unschedulable taint with Exists operator", func() {
			pod := &corev1.Pod{Spec: corev1.PodSpec{
				Tolerations: []corev1.Toleration{{
					Key:      corev1.TaintNodeUnschedulable,
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip pods with a wildcard NoSchedule toleration", func() {
			pod := &corev1.Pod{Spec: corev1.PodSpec{
				Tolerations: []corev1.Toleration{{
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should not skip a normal pod", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"}}
			Expect(ShouldSkipPod(pod)).To(BeFalse())
		})
	})

	Describe("#NodeIsUnavailableForInPlaceUpdate", func() {
		It("should return true when the node is unschedulable and has the drain start annotation", func() {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime: time.Now().Format(time.RFC3339)},
				},
				Spec: corev1.NodeSpec{Unschedulable: true},
			}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeTrue())
		})

		It("should return true when the node is unschedulable and has a non-successful in-place update condition", func() {
			node := &corev1.Node{
				Spec: corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
					Type:   machinev1alpha1.NodeInPlaceUpdate,
					Status: corev1.ConditionTrue,
					Reason: machinev1alpha1.ReadyForUpdate,
				}}},
			}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeTrue())
		})

		It("should return true when the node has the failed update-result label", func() {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					machinev1alpha1.LabelKeyNodeUpdateResult: machinev1alpha1.LabelValueNodeUpdateFailed,
				},
			}}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeTrue())
		})

		It("should return false for a schedulable node with no condition or label", func() {
			node := &corev1.Node{}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeFalse())
		})

		It("should return false for a successfully updated node (only the successful condition)", func() {
			node := &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
				Type:   machinev1alpha1.NodeInPlaceUpdate,
				Status: corev1.ConditionTrue,
				Reason: machinev1alpha1.UpdateSuccessful,
			}}}}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeFalse())
		})
	})

	Describe("#MaxUnavailableForPool", func() {
		It("should return 1 when the pool is not found", func() {
			Expect(MaxUnavailableForPool(nil, "unknown", 5)).To(Equal(1))
		})

		It("should return 1 when MaxUnavailable is nil", func() {
			workers := []gardencorev1beta1.Worker{{Name: poolName, Maximum: 5}}
			Expect(MaxUnavailableForPool(workers, poolName, 5)).To(Equal(1))
		})

		It("should return the absolute MaxUnavailable", func() {
			workers := []gardencorev1beta1.Worker{{
				Name:           poolName,
				Maximum:        5,
				MaxUnavailable: new(intstr.FromInt(2)),
			}}
			Expect(MaxUnavailableForPool(workers, poolName, 5)).To(Equal(2))
		})

		It("should always return 1 for control-plane pools", func() {
			workers := []gardencorev1beta1.Worker{{
				Name:           poolName,
				Maximum:        5,
				MaxUnavailable: new(intstr.FromInt(3)),
				ControlPlane:   &gardencorev1beta1.WorkerControlPlane{},
			}}
			Expect(MaxUnavailableForPool(workers, poolName, 5)).To(Equal(1))
		})

		It("should scale percentage against current node count", func() {
			workers := []gardencorev1beta1.Worker{{
				Name:           poolName,
				Maximum:        10,
				MaxUnavailable: new(intstr.FromString("50%")),
			}}
			Expect(MaxUnavailableForPool(workers, poolName, 2)).To(Equal(1))
			Expect(MaxUnavailableForPool(workers, poolName, 4)).To(Equal(2))
		})
	})
})
