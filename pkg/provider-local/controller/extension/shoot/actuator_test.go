// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
)

var _ = Describe("triggerKubeProxyManagedResourceReconciliation", func() {
	const namespace = "shoot--test--foo"

	var (
		ctx        context.Context
		fakeClient client.Client
		a          *actuator
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesclient.SeedScheme).Build()
		a = &actuator{client: fakeClient}
	})

	It("should be a no-op when no kube-proxy ManagedResources exist", func() {
		Expect(a.triggerKubeProxyManagedResourceReconciliation(ctx, namespace)).To(Succeed())
	})

	It("should annotate all kube-proxy ManagedResources with operation=reconcile", func() {
		// Create two kube-proxy ManagedResources and one unrelated one.
		kubeProxyMR1 := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-kube-proxy",
				Namespace: namespace,
				Labels:    kubeproxy.ManagedResourceLabelSelector(),
			},
		}
		kubeProxyMR2 := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-kube-proxy-workers-v1.36.0",
				Namespace: namespace,
				Labels:    kubeproxy.ManagedResourceLabelSelector(),
			},
		}
		otherMR := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-other",
				Namespace: namespace,
				Labels:    map[string]string{"component": "other"},
			},
		}

		Expect(fakeClient.Create(ctx, kubeProxyMR1)).To(Succeed())
		Expect(fakeClient.Create(ctx, kubeProxyMR2)).To(Succeed())
		Expect(fakeClient.Create(ctx, otherMR)).To(Succeed())

		Expect(a.triggerKubeProxyManagedResourceReconciliation(ctx, namespace)).To(Succeed())

		// kube-proxy ManagedResources must have the reconcile annotation.
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(kubeProxyMR1), kubeProxyMR1)).To(Succeed())
		Expect(kubeProxyMR1.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(kubeProxyMR2), kubeProxyMR2)).To(Succeed())
		Expect(kubeProxyMR2.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))

		// Unrelated ManagedResource must not be touched.
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(otherMR), otherMR)).To(Succeed())
		Expect(otherMR.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
	})

	It("should not annotate kube-proxy ManagedResources in other namespaces", func() {
		mrOtherNS := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-kube-proxy",
				Namespace: "other-namespace",
				Labels:    kubeproxy.ManagedResourceLabelSelector(),
			},
		}
		Expect(fakeClient.Create(ctx, mrOtherNS)).To(Succeed())

		Expect(a.triggerKubeProxyManagedResourceReconciliation(ctx, namespace)).To(Succeed())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mrOtherNS), mrOtherNS)).To(Succeed())
		Expect(mrOtherNS.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
	})
})
