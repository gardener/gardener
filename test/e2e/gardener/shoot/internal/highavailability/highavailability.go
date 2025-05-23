// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailability

import (
	"context"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// VerifyHighAvailability verifies the high-availability settings of a shoot cluster. It checks the topology spread
// constraints, ETCD affinity, and envoy filters to ensure the shoot cluster is correctly updated to high-availability
// configuration.
func VerifyHighAvailability(s *ShootContext) {
	GinkgoHelper()

	Describe("verify high-availability", func() {
		It("should verify the topology spread constraints", func(ctx SpecContext) {
			verifyTopologySpreadConstraints(ctx, s.SeedKomega, s.Shoot.Status.TechnicalID, s.Shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type)
		}, SpecTimeout(time.Minute))

		It("should verify the ETCD affinity", func(ctx SpecContext) {
			verifyETCDAffinity(ctx, s.SeedClient, s.Shoot.Status.TechnicalID, s.Shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type)
		}, SpecTimeout(time.Minute))

		It("should verify the envoy filters", func(ctx SpecContext) {
			verifyEnvoyFilters(ctx, s.SeedClient, s.Shoot.Status.TechnicalID)
		}, SpecTimeout(time.Minute))
	})
}

func verifyTopologySpreadConstraints(ctx context.Context, seedKomega komega.Komega, shootTechnicalID string, failureToleranceType gardencorev1beta1.FailureToleranceType) {
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

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: shootTechnicalID}}
	Eventually(ctx, seedKomega.Object(deployment)).Should(HaveField("Spec.Template.Spec.TopologySpreadConstraints", matcher))
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
		}).Should(Succeed(), "etcd %s should have node affinity to %d zones", name, numberOfZones)
	}
}

func verifyEnvoyFilters(ctx context.Context, seedClient client.Client, shootTechnicalID string) {
	GinkgoHelper()

	shootFilterNames := sets.New(
		shootTechnicalID+"-apiserver-proxy",
		shootTechnicalID+"-istio-tls-termination",
	)

	Eventually(ctx, func(g Gomega) {
		envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
		g.Expect(seedClient.List(ctx, envoyFilterList)).To(Succeed())

		envoyFilterList.Items = slices.DeleteFunc(envoyFilterList.Items, func(envoyFilter *istionetworkingv1alpha3.EnvoyFilter) bool {
			return !shootFilterNames.Has(envoyFilter.Name)
		})

		// TODO(Wieneo,oliver-goetz): Change or remove this once the feature gates "IstioTLSTermination" or
		//  "RemoveAPIServerProxyLegacyPort" are removed.
		// We can have the following scenarios:
		// - Feature Gate "IstioTLSTermination" enabled,  "RemoveAPIServerProxyLegacyPort" disabled = 2 envoy filters
		// - Feature Gate "IstioTLSTermination" enabled,  "RemoveAPIServerProxyLegacyPort" enabled  = 2 envoy filters
		// - Feature Gate "IstioTLSTermination" disabled, "RemoveAPIServerProxyLegacyPort" disabled = 1 envoy filter
		// - Feature Gate "IstioTLSTermination" disabled, "RemoveAPIServerProxyLegacyPort" enabled  = 0 envoy filters

		// Istio TLS termination is considered to be active when `kube-apiserver-mtls` service exists in the shoot namespace.
		istioTLSTerminationEnabled := true
		err := seedClient.Get(ctx, client.ObjectKey{Name: "kube-apiserver-mtls", Namespace: shootTechnicalID}, &corev1.Service{})
		if apierrors.IsNotFound(err) {
			istioTLSTerminationEnabled = false
		} else {
			g.Expect(err).NotTo(HaveOccurred())
		}

		// "RemoveAPIServerProxyLegacyPort" is considered to be active when the "istio-ingressgateway" has no 8443 port open
		istioIngressGatewayService := &corev1.Service{}
		g.Expect(seedClient.Get(ctx, client.ObjectKey{Name: v1beta1constants.DefaultSNIIngressServiceName, Namespace: v1beta1constants.DefaultSNIIngressNamespace}, istioIngressGatewayService)).To(Succeed())

		removeAPIServerProxyLegacyPortEnabled := true
		for _, k := range istioIngressGatewayService.Spec.Ports {
			if k.Port == 8443 {
				removeAPIServerProxyLegacyPortEnabled = false
			}
		}

		// assume that "IstioTLSTermination" is enabled, default expected envoy filters to 2
		expectedEnvoyFilters := 2

		// the number of envoy filters can only change if "IstioTLSTermination" is disabled
		if !istioTLSTerminationEnabled {
			if !removeAPIServerProxyLegacyPortEnabled {
				expectedEnvoyFilters = 1
			} else {
				expectedEnvoyFilters = 0
			}
		}

		g.Expect(envoyFilterList.Items).To(HaveLen(expectedEnvoyFilters))
		for _, envoyFilter := range envoyFilterList.Items {
			g.Expect(envoyFilter.Namespace).To(HavePrefix("istio-ingress"), "for envoy filter %s", client.ObjectKeyFromObject(envoyFilter))
		}
	}).Should(Succeed())
}
