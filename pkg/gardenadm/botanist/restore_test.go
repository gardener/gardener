// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Restore", func() {
	const priorNodeName = "prior-node"

	var (
		b *GardenadmBotanist

		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
			Build()
		fakeClientSet := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Logger:        logr.Discard(),
					SeedClientSet: fakeClientSet,
				},
			},
		}
	})

	Describe("#DeletePriorNode", func() {
		It("should not fail if the prior Node does not exist", func(ctx SpecContext) {
			Expect(b.DeletePriorNode(ctx, priorNodeName)).To(Succeed())
		})

		It("should delete the prior Node", func(ctx SpecContext) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: priorNodeName}}
			other := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "other-node"}}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			Expect(fakeClient.Create(ctx, other)).To(Succeed())

			Expect(b.DeletePriorNode(ctx, priorNodeName)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(other), other)).To(Succeed())
		})
	})

	Describe("#ForceDeletePriorNodePods", func() {
		podOnNode := func(name, namespace, nodeName string) *corev1.Pod {
			return &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       corev1.PodSpec{NodeName: nodeName},
			}
		}

		It("should not fail if there are no Pods running on the prior Node", func(ctx SpecContext) {
			Expect(b.ForceDeletePriorNodePods(ctx, priorNodeName)).To(Succeed())
		})

		It("should delete all Pods running on the prior Node", func(ctx SpecContext) {
			pod1 := podOnNode("pod-1", "kube-system", priorNodeName)
			pod2 := podOnNode("pod-2", "default", priorNodeName)
			pod3 := podOnNode("pod-3", "some-namespace", priorNodeName)
			pod4 := podOnNode("pod-4", "kube-system", "other-node")
			pod5 := podOnNode("pod-5", "kube-system", "")
			Expect(fakeClient.Create(ctx, pod1)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod2)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod3)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod4)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod5)).To(Succeed())

			Expect(b.ForceDeletePriorNodePods(ctx, priorNodeName)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod1), pod1)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod2), pod2)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod3), pod3)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod4), pod4)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod5), pod5)).To(Succeed())
		})
	})
})
