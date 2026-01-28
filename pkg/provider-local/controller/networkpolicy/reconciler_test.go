// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/provider-local/controller/networkpolicy"
)

const testExposureclass = "exposureclass"

var _ = Describe("Reconciler", func() {
	var (
		r *Reconciler
		k komega.Komega
		c client.Client
	)
	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		k = komega.New(c)
		r = &Reconciler{
			Client: c,
		}
	})

	DescribeTable("#Reconcile", func(ctx context.Context, shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed, expectedPeers []networkingv1.NetworkPolicyPeer) {
		k = k.WithContext(ctx)
		cluster := defaultCluster(shoot, seed)
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name}}
		exposureclassNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testExposureclass, Labels: map[string]string{constants.LabelExposureClassHandlerName: testExposureclass}}}
		Expect(c.Create(ctx, namespace)).NotTo(HaveOccurred())
		Expect(c.Create(ctx, exposureclassNamespace)).NotTo(HaveOccurred())
		Expect(c.Create(ctx, cluster)).NotTo(HaveOccurred())

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(namespace)})
		Expect(err).NotTo(HaveOccurred(), "reconcile")

		netpol := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-to-istio-ingress-gateway",
				Namespace: namespace.Name,
			},
		}

		expected := netpol.DeepCopy()
		expected.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
			{
				To: expectedPeers,
			},
		}
		Expect(k.Object(netpol)()).To(komega.EqualObject(expected, komega.MatchPaths{"spec.egress[0].to"}))
	},
		Entry("Cluster",
			&gardencorev1beta1.Shoot{},
			&gardencorev1beta1.Seed{},
			[]networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "istio-ingress",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":   "istio-ingressgateway",
							"istio": "ingressgateway",
						},
					},
				},
			},
		),
		Entry("Cluster with zones in seed",
			&gardencorev1beta1.Shoot{},
			&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Zones: []string{"1", "2", "3"},
					},
				},
			},
			[]networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "istio-ingress",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":   "istio-ingressgateway",
							"istio": "ingressgateway",
						},
					},
				},
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "istio-ingress--1",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":   "istio-ingressgateway",
							"istio": "ingressgateway--zone--1",
						},
					},
				},
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "istio-ingress--2",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":   "istio-ingressgateway",
							"istio": "ingressgateway--zone--2",
						},
					},
				},
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "istio-ingress--3",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":   "istio-ingressgateway",
							"istio": "ingressgateway--zone--3",
						},
					},
				},
			},
		),
		Entry("Cluster with exposure class",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					ExposureClassName: ptr.To(testExposureclass),
				},
			},
			&gardencorev1beta1.Seed{},
			[]networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "kubernetes.io/metadata.name",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{testExposureclass},
							},
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "istio-ingressgateway",
							"handler.exposureclass.gardener.cloud/name": testExposureclass,
						},
					},
				},
			},
		),
	)
})

func defaultCluster(shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed) *extensionsv1alpha1.Cluster {
	GinkgoHelper()

	encoder := json.NewSerializerWithOptions(json.DefaultMetaFactory, kubernetes.SeedScheme, kubernetes.SeedScheme, json.SerializerOptions{Yaml: false, Pretty: false, Strict: false})
	rawSeed, err := runtime.Encode(encoder, seed)
	Expect(err).NotTo(HaveOccurred())

	rawShoot, err := runtime.Encode(encoder, shoot)
	Expect(err).NotTo(HaveOccurred())

	return &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mycluster",
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			Seed: runtime.RawExtension{
				Raw: rawSeed,
			},
			Shoot: runtime.RawExtension{
				Raw: rawShoot,
			},
		},
	}
}
