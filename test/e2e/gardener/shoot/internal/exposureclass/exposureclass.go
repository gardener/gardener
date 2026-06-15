// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package exposureclass

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
)

// VerifyExposureClassSwitch verifies the switch of exposure class of a shoot cluster.
// It checks the connectivity to the API server both with exposure class and without exposure class set
// while waiting that the cluster gets healthy after the exposure class switch.
func VerifyExposureClassSwitch(s *ShootContext, waitForReconcileFunc func(s *ShootContext)) {
	GinkgoHelper()
	defer GinkgoRecover()

	expClass := &gardencorev1beta1.ExposureClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "local",
		},
	}

	Describe("switching exposure class", Ordered, func() {
		BeforeAll(func(ctx context.Context) {
			if err := s.GardenClient.Get(ctx, client.ObjectKeyFromObject(expClass), expClass); apierrors.IsNotFound(err) {
				Skip("exposure class not installed")
			}
		})

		verifyAPIServerAccess := func() {
			It("should be able to talk to API server", func(ctx SpecContext) {
				Eventually(ctx, s.ShootKomega.Get(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}})).Should(Succeed())
			}, SpecTimeout(3*time.Minute)) // timeout must be greater than the ttl of the dnsrecord
			if !v1beta1helper.IsWorkerless(s.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(s)
			}
		}

		It("Switch exposure class", func(ctx SpecContext) {
			Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
				s.Shoot.Spec.ExposureClassName = &expClass.Name
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
