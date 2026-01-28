package exposureclass

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
)

func VerifyExposureClassSwitch(s *ShootContext, waitForReconcileFunc func(s *ShootContext)) {
	GinkgoHelper()

	// TODO: Layer 7 Loadbalancing is currently not working with switching exposure classes
	// as the aliases for the API server is not updated correctly on the controlplane components (controller-manager, gardener-resource-manager etc.)
	if s.Shoot.Annotations[v1beta1constants.ShootDisableIstioTLSTermination] == "true" {
		Describe("switching exposure class", func() {
			verifyAPIServerAccess := func() {
				It("should be able to talk to API server", func(ctx SpecContext) {
					Eventually(ctx, s.ShootKomega.Get(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}})).Should(Succeed())
				}, SpecTimeout(time.Minute))
				if !v1beta1helper.IsWorkerless(s.Shoot) {
					inclusterclient.VerifyInClusterAccessToAPIServer(s)
				}
			}

			It("Switch exposure class", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
					s.Shoot.Spec.ExposureClassName = ptr.To("exposureclass")
				})).Should(Succeed())
			}, SpecTimeout(time.Minute))
			waitForReconcileFunc(s)
			Describe("with exposure class", verifyAPIServerAccess)

			It("Without exposure class", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
					s.Shoot.Spec.ExposureClassName = nil
				})).Should(Succeed())
			}, SpecTimeout(time.Minute))
			waitForReconcileFunc(s)
			Describe("Without exposure class", verifyAPIServerAccess)
		})
	}
}
