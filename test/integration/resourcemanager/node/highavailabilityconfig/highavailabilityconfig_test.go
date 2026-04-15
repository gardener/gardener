// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Node HighAvailabilityConfig controller", func() {
	var (
		namespace *corev1.Namespace
		labels    = map[string]string{"app": "test"}
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Labels: map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigConsider: "true",
				},
				Annotations: map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType: "node",
					resourcesv1alpha1.HighAvailabilityConfigZones:                "a",
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create test namespace")
		Expect(testClient.Create(ctx, namespace)).To(Succeed())
		log.Info("Created Namespace", "namespaceName", namespace.Name)

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, namespace)).To(Succeed())
		})
	})

	test := func(obj client.Object, getTSC func() []corev1.TopologySpreadConstraint) {
		By("Create first node")
		node1 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testIDPrefix + "-node-1-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]}}
		Expect(testClient.Create(ctx, node1)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, node1)).To(Succeed())
		})

		By("Create object with 1 node (expect ScheduleAnyway)")
		Expect(testClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, obj)).To(Succeed())
		})

		Expect(getTSC()).To(ContainElement(HaveField("WhenUnsatisfiable", corev1.ScheduleAnyway)))

		By("Create second node (controller should patch, webhook switches to DoNotSchedule)")
		node2 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testIDPrefix + "-node-2-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]}}
		Expect(testClient.Create(ctx, node2)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, node2))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			g.Expect(getTSC()).To(ContainElement(HaveField("WhenUnsatisfiable", corev1.DoNotSchedule)))
		}).Should(Succeed())

		By("Delete second node (controller should patch, webhook switches back to ScheduleAnyway)")
		Expect(testClient.Delete(ctx, node2)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(getTSC()).To(ContainElement(HaveField("WhenUnsatisfiable", corev1.ScheduleAnyway)))
		}).Should(Succeed())
	}

	It("should update topology spread constraints for Deployments when node count crosses the single-node threshold", func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deploy-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
				Labels: map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Replicas: ptr.To[int32](2),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
			},
		}

		test(deployment, func() []corev1.TopologySpreadConstraint {
			deploy := &appsv1.Deployment{}
			ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deploy)).To(Succeed())
			return deploy.Spec.Template.Spec.TopologySpreadConstraints
		})
	})

	It("should update topology spread constraints for StatefulSets when node count crosses the single-node threshold", func() {
		statefulSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sts-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
				Labels: map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Replicas: ptr.To[int32](2),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
			},
		}

		test(statefulSet, func() []corev1.TopologySpreadConstraint {
			sts := &appsv1.StatefulSet{}
			ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(statefulSet), sts)).To(Succeed())
			return sts.Spec.Template.Spec.TopologySpreadConstraints
		})
	})
})
