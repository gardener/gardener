// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("gardenadm Tests", Label("gardenadm", "default"), func() {
	BeforeEach(OncePerOrdered, func(ctx SpecContext) {
		By("Ensuring fresh machine pods for test execution")
		Expect(runtimeClient.Client().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(namespace), client.MatchingLabels{"app": "machine"})).To(Succeed())

		Eventually(ctx, func(g Gomega) error {
			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: statefulSetName, Namespace: namespace}}
			g.Expect(runtimeClient.Client().Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(Succeed())
			return health.CheckStatefulSet(statefulSet)
		}).Should(Succeed())
	}, NodeTimeout(time.Minute))

	Describe("Single-node control plane", Ordered, Label("single"), func() {
		It("should initialize the control plane node", func(ctx SpecContext) {
			stdout, _, err := runtimeClient.PodExecutor().Execute(ctx, namespace, machinePodName(0), containerName,
				"gardenadm", "init",
			)
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, gbytes.BufferReader(stdout)).Should(gbytes.Say("not implemented"))
		}, SpecTimeout(5*time.Second))

		It("should join the worker node", func(ctx SpecContext) {
			stdout, _, err := runtimeClient.PodExecutor().Execute(ctx, namespace, machinePodName(1), containerName,
				"gardenadm", "join",
			)
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, gbytes.BufferReader(stdout)).Should(gbytes.Say("not implemented either"))
		}, SpecTimeout(5*time.Second))
	})
})
