// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/controller/infrastructure"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(tc *ShootContext) {
		// Preselecting fast deletion must not change creation or reconciliation.
		metav1.SetMetaDataAnnotation(&tc.Shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationFastDeletion, "true")
		BeforeAll(func() { tc.Init() })

		Describe("Create and Fast Delete Shoot with unavailable etcd and kube-apiserver", Label("fast-delete"), func() {
			ItShouldCreateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldGetResponsibleSeed(tc)
			seed.ItShouldInitializeSeedClient(tc.SeedContext)
			ItShouldAnnotateShoot(tc, map[string]string{
				v1beta1constants.ShootIgnore:                        "true",
				v1beta1constants.AnnotationConfirmationFastDeletion: "false",
			})

			It("should stop kube-apiserver and both etcds", func(ctx SpecContext) {
				namespace := tc.Shoot.Status.TechnicalID
				Expect(kubernetesutils.ScaleDeployment(ctx, tc.SeedClient,
					client.ObjectKey{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, 0,
				)).To(Succeed())
				for _, name := range []string{"etcd-main", "etcd-events"} {
					etcd := &druidcorev1alpha1.Etcd{}
					Expect(tc.SeedClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, etcd)).To(Succeed())
					patch := client.MergeFrom(etcd.DeepCopy())
					etcd.Spec.Replicas = 0
					metav1.SetMetaDataAnnotation(&etcd.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
					Expect(tc.SeedClient.Patch(ctx, etcd, patch)).To(Succeed())
				}
				Eventually(ctx, tc.SeedKomega.ObjectList(&corev1.PodList{}, client.InNamespace(namespace),
					client.MatchingLabels{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameKubeAPIServer},
				)).Should(HaveField("Items", BeEmpty()))
				for _, role := range []string{"main", "events"} {
					Eventually(ctx, tc.SeedKomega.ObjectList(&corev1.PodList{}, client.InNamespace(namespace),
						client.MatchingLabels{"app": "etcd-statefulset", "role": role},
					)).Should(HaveField("Items", BeEmpty()))
				}
			}, SpecTimeout(5*time.Minute))

			ItShouldDeleteShoot(tc)
			ItShouldAnnotateShoot(tc, map[string]string{
				v1beta1constants.ShootIgnore:                        "false",
				v1beta1constants.AnnotationConfirmationFastDeletion: "true",
			})

			It("should delete without restoring etcd or kube-apiserver", func(ctx SpecContext) {
				var restored bool
				Eventually(ctx, func(g Gomega) bool {
					deployment := &appsv1.Deployment{}
					err := tc.SeedClient.Get(ctx, client.ObjectKey{Namespace: tc.Shoot.Status.TechnicalID, Name: v1beta1constants.DeploymentNameKubeAPIServer}, deployment)
					g.Expect(client.IgnoreNotFound(err)).To(Succeed())
					if err == nil && deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0 {
						restored = true
					}
					for _, name := range []string{"etcd-main", "etcd-events"} {
						etcd := &druidcorev1alpha1.Etcd{}
						err := tc.SeedClient.Get(ctx, client.ObjectKey{Namespace: tc.Shoot.Status.TechnicalID, Name: name}, etcd)
						g.Expect(client.IgnoreNotFound(err)).To(Succeed())
						if err == nil && etcd.Spec.Replicas > 0 {
							restored = true
						}
					}
					err = tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(tc.Shoot), tc.Shoot.DeepCopy())
					g.Expect(client.IgnoreNotFound(err)).To(Succeed())
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
				Expect(restored).To(BeFalse(), "fast deletion must not restore the control plane")
			}, SpecTimeout(20*time.Minute))

			It("should delete Seed resources and the underlying worker infrastructure", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					return tc.SeedClient.Get(ctx, client.ObjectKey{Name: tc.Shoot.Status.TechnicalID}, &corev1.Namespace{})
				}).Should(BeNotFoundError())
				if len(tc.Shoot.Spec.Provider.Workers) > 0 {
					Eventually(ctx, func() error {
						return tc.SeedClient.Get(ctx, client.ObjectKey{Name: infrastructure.NamespaceName(tc.Shoot.Status.TechnicalID)}, &corev1.Namespace{})
					}).Should(BeNotFoundError())
				}
			}, SpecTimeout(5*time.Minute))
		})
	}

	Context("Shoot with workers", Ordered, func() {
		test(NewShootContext(DefaultShoot("e2e-fast-del")))
	})
	Context("Workerless Shoot", Label("workerless"), Ordered, func() {
		test(NewShootContext(DefaultWorkerlessShoot("e2e-fast-del")))
	})
})
