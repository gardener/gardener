// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
)

var _ = Describe("CriticalStaticPods", func() {
	var (
		ctx = context.Background()

		fakeClient client.Client
		fakeClock  *testing.FakeClock
		checker    *CriticalStaticPodsChecker

		node *corev1.Node
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithStatusSubresource(&corev1.Node{}).
			WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
			Build()

		fakeClock = testing.NewFakeClock(time.Now().Truncate(time.Second))
		checker = NewCriticalStaticPodsChecker(fakeClient, fakeClock)

		node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}}
		Expect(fakeClient.Create(ctx, node)).To(Succeed())
	})

	criticalStaticPod := func(name, nodeName string, phase corev1.PodPhase, ready corev1.ConditionStatus) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceSystem,
				Labels:    map[string]string{"shoot.gardener.cloud/is-critical-static-pod": "true"},
			},
			Spec: corev1.PodSpec{NodeName: nodeName},
			Status: corev1.PodStatus{
				Phase:      phase,
				Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: ready}},
			},
		}
	}

	condition := func() *corev1.NodeCondition {
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		for i, c := range node.Status.Conditions {
			if c.Type == "CriticalControlPlanePodsUnhealthy" {
				return &node.Status.Conditions[i]
			}
		}
		return nil
	}

	It("should report healthy when there are no critical static pods", func() {
		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status": Equal(corev1.ConditionFalse),
			"Reason": Equal("AllCriticalControlPlanePodsHealthy"),
		})))
	})

	It("should report healthy when the critical static pod is running and ready", func() {
		Expect(fakeClient.Create(ctx, criticalStaticPod("etcd-main", "node", corev1.PodRunning, corev1.ConditionTrue))).To(Succeed())

		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status": Equal(corev1.ConditionFalse),
			"Reason": Equal("AllCriticalControlPlanePodsHealthy"),
		})))
	})

	It("should report unhealthy when the critical static pod is in an invalid phase", func() {
		Expect(fakeClient.Create(ctx, criticalStaticPod("etcd-main", "node", corev1.PodPending, corev1.ConditionTrue))).To(Succeed())

		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(corev1.ConditionTrue),
			"Reason":  Equal("SomeCriticalControlPlanePodsUnhealthy"),
			"Message": ContainSubstring("invalid phase"),
		})))
	})

	It("should report unhealthy when the critical static pod is running but not ready", func() {
		Expect(fakeClient.Create(ctx, criticalStaticPod("etcd-main", "node", corev1.PodRunning, corev1.ConditionFalse))).To(Succeed())

		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(corev1.ConditionTrue),
			"Reason":  Equal("SomeCriticalControlPlanePodsUnhealthy"),
			"Message": Equal("critical static pod kube-system/etcd-main is not ready"),
		})))
	})

	It("should ignore pods without the critical static pod label", func() {
		pod := criticalStaticPod("some-pod", "node", corev1.PodPending, corev1.ConditionFalse)
		pod.Labels = nil
		Expect(fakeClient.Create(ctx, pod)).To(Succeed())

		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status": Equal(corev1.ConditionFalse),
		})))
	})

	It("should ignore critical static pods running on other nodes", func() {
		Expect(fakeClient.Create(ctx, criticalStaticPod("etcd-main", "other-node", corev1.PodPending, corev1.ConditionFalse))).To(Succeed())

		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status": Equal(corev1.ConditionFalse),
		})))
	})

	It("should only update the last transition time when the status changes", func() {
		Expect(checker.Check(ctx, node)).To(Succeed())
		transitionTime := condition().LastTransitionTime

		fakeClock.Step(time.Minute)
		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition().LastTransitionTime).To(Equal(transitionTime))
		Expect(condition().LastHeartbeatTime.Time).To(BeTemporally("==", fakeClock.Now()))

		fakeClock.Step(time.Minute)
		Expect(fakeClient.Create(ctx, criticalStaticPod("etcd-main", "node", corev1.PodRunning, corev1.ConditionFalse))).To(Succeed())
		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(condition().Status).To(Equal(corev1.ConditionTrue))
		Expect(condition().LastTransitionTime.Time).To(BeTemporally("==", fakeClock.Now()))
	})

	It("should preserve conditions managed by other components", func() {
		node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}}
		Expect(fakeClient.Status().Update(ctx, node)).To(Succeed())

		Expect(checker.Check(ctx, node)).To(Succeed())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		Expect(node.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Type": Equal(corev1.NodeNetworkUnavailable),
		})))
		Expect(condition()).NotTo(BeNil())
	})
})
