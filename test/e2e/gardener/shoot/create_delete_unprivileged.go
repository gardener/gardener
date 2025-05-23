// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create and Delete Unprivileged Shoot", Ordered, Label("unprivileged", "basic"), func() {
		var s *ShootContext

		BeforeTestSetup(func() {
			shoot := DefaultShoot("e2e-unpriv")
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name: "PodSecurity",
					Config: &runtime.RawExtension{
						Raw: []byte(`{
  "apiVersion": "pod-security.admission.config.k8s.io/v1beta1",
  "kind": "PodSecurityConfiguration",
  "defaults": {
    "enforce": "restricted",
    "enforce-version": "latest"
  }
}`),
					},
				},
			}

			s = NewTestContext().ForShoot(shoot)
		})

		ItShouldCreateShoot(s)
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldInitializeShootClient(s)

		It("should allow creating pod in the kube-system namespace", func(ctx SpecContext) {
			pod := newPodForNamespace(metav1.NamespaceSystem)

			DeferCleanup(func(ctx SpecContext) {
				Eventually(ctx, func() error {
					return s.ShootClient.Delete(ctx, pod)
				}).Should(Or(Succeed(), BeNotFoundError()))
			}, NodeTimeout(time.Minute))

			Eventually(ctx, func() error {
				return s.ShootClient.Create(ctx, pod)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("should forbid creating pod in the default namespace", func(ctx SpecContext) {
			pod := newPodForNamespace(metav1.NamespaceDefault)

			DeferCleanup(func(ctx SpecContext) {
				// ensure test step leaves clean state even in the case of failure (the pod is allowed to be created)
				Eventually(ctx, func() error {
					return s.ShootClient.Delete(ctx, pod)
				}).Should(Or(Succeed(), BeNotFoundError()))
			}, NodeTimeout(time.Minute))

			Eventually(ctx, func() error {
				if err := s.ShootClient.Create(ctx, pod); err != nil {
					return err
				}
				return StopTrying("pod was created")
			}).Should(And(
				BeForbiddenError(),
				MatchError(ContainSubstring("pods %q is forbidden: violates PodSecurity %q", "nginx", "restricted:latest")),
			))
		}, SpecTimeout(time.Minute))

		inclusterclient.VerifyInClusterAccessToAPIServer(s)

		ItShouldDeleteShoot(s)
		ItShouldWaitForShootToBeDeleted(s)
	})
})

func newPodForNamespace(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.14.2",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}
}
