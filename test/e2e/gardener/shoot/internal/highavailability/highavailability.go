// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailability

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// VerifyHighAvailabilityUpdate verifies the high-availability update of a shoot cluster. It checks the topology
// spread constraints, ETCD affinity, and envoy filters to ensure the shoot cluster is correctly upgraded to
// high-availability configuration.
func VerifyHighAvailabilityUpdate(s *ShootContext) {
	GinkgoHelper()

	Describe("high-availability upgrade", func() {
		It("should verify the topology spread constraints", func(ctx SpecContext) {
			verifyTopologySpreadConstraints(ctx, s.SeedClient, s.Shoot.Status.TechnicalID, s.Shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type)
		}, SpecTimeout(time.Minute))

		It("should verify the ETCD affinity", func(ctx SpecContext) {
			verifyETCDAffinity(ctx, s.SeedClient, s.Shoot.Status.TechnicalID, s.Shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type)
		}, SpecTimeout(time.Minute))

		It("should verify the envoy filters", func(ctx SpecContext) {
			verifyEnvoyFilters(ctx, s.SeedClient, s.Shoot.Status.TechnicalID)
		}, SpecTimeout(time.Minute))
	})
}

func verifyTopologySpreadConstraints(ctx context.Context, seedClient client.Client, shootTechnicalID string, failureToleranceType gardencorev1beta1.FailureToleranceType) {
	GinkgoHelper()

	var (
		nodeSpread = MatchFields(IgnoreExtras, Fields{
			"MaxSkew":           Equal(int32(1)),
			"TopologyKey":       Equal(corev1.LabelHostname),
			"WhenUnsatisfiable": Equal(corev1.DoNotSchedule),
		})
		zoneSpread = MatchFields(IgnoreExtras, Fields{
			"MaxSkew":           Equal(int32(1)),
			"TopologyKey":       Equal(corev1.LabelTopologyZone),
			"WhenUnsatisfiable": Equal(corev1.DoNotSchedule),
		})
		matcher gomegatypes.GomegaMatcher
	)

	switch failureToleranceType {
	case gardencorev1beta1.FailureToleranceTypeNode:
		matcher = ConsistOf(nodeSpread)
	case gardencorev1beta1.FailureToleranceTypeZone:
		matcher = ConsistOf(nodeSpread, zoneSpread)
	default:
		matcher = BeNil()
	}

	Eventually(ctx, func(g Gomega) {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: shootTechnicalID}}
		g.Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

		g.Expect(deployment.Spec.Template.Spec.TopologySpreadConstraints).To(matcher)
	}).Should(Succeed())
}

func verifyETCDAffinity(ctx context.Context, seedClient client.Client, shootTechnicalID string, failureToleranceType gardencorev1beta1.FailureToleranceType) {
	GinkgoHelper()

	numberOfZones := 1
	if failureToleranceType == gardencorev1beta1.FailureToleranceTypeZone {
		numberOfZones = 3
	}

	for _, name := range []string{v1beta1constants.ETCDRoleEvents, v1beta1constants.ETCDRoleMain} {
		Eventually(ctx, func(g Gomega) {
			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + name, Namespace: shootTechnicalID}}
			g.Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(Succeed())

			g.Expect(statefulSet.Spec.Template.Spec.Affinity).NotTo(BeNil())
			g.Expect(statefulSet.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
			g.Expect(statefulSet.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).NotTo(BeNil())
			g.Expect(statefulSet.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(HaveLen(1))
			g.Expect(statefulSet.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions).To(HaveLen(1))
			g.Expect(statefulSet.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Key":      Equal(corev1.LabelTopologyZone),
				"Operator": Equal(corev1.NodeSelectorOpIn),
				"Values":   HaveLen(numberOfZones),
			}))
			g.Expect(statefulSet.Spec.Template.Spec.Affinity.PodAntiAffinity).To(BeNil())
		}).Should(Succeed(), " for etcd "+name)
	}
}

func verifyEnvoyFilters(ctx context.Context, seedClient client.Client, shootTechnicalID string) {
	GinkgoHelper()

	Eventually(ctx, func(g Gomega) {
		envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
		g.Expect(seedClient.List(ctx, envoyFilterList, client.MatchingFields{metav1.ObjectNameField: shootTechnicalID})).To(Succeed())

		g.Expect(envoyFilterList.Items).To(HaveLen(1))
		g.Expect(envoyFilterList.Items[0].Namespace).To(HavePrefix("istio-ingress"))
	}).Should(Succeed())
}
